package repository

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"math"
	"sort"
)

type SQLiteVectorRepository struct {
	db *sql.DB
}

func NewSQLiteVectorRepository(db *sql.DB) *SQLiteVectorRepository {
	return &SQLiteVectorRepository{db: db}
}

func (r *SQLiteVectorRepository) StoreEmbedding(ctx context.Context, sourceType string, sourceID int64, model string, embedding []float32) error {
	serialized := serializeFloat32(embedding)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO embeddings(source_type, source_id, model, dimensions, embedding_blob, created_at)
		 VALUES(?, ?, ?, ?, ?, datetime('now'))
		 ON CONFLICT(source_type, source_id, model) DO UPDATE SET
		 	dimensions = excluded.dimensions,
		 	embedding_blob = excluded.embedding_blob,
		 	created_at = datetime('now')`,
		sourceType, sourceID, model, len(embedding), serialized,
	)
	if err != nil {
		return err
	}
	return nil
}

func (r *SQLiteVectorRepository) SearchSimilar(ctx context.Context, sourceType string, queryEmbedding []float32, limit int) ([]VectorMatch, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT source_type, source_id, embedding_blob
		 FROM embeddings
		 WHERE source_type = ? AND dimensions = ? AND length(embedding_blob) > 0`,
		sourceType, len(queryEmbedding),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []VectorMatch
	for rows.Next() {
		var m VectorMatch
		var blob []byte
		if err := rows.Scan(&m.SourceType, &m.SourceID, &blob); err != nil {
			return nil, err
		}
		embedding := deserializeFloat32(blob)
		if len(embedding) != len(queryEmbedding) {
			continue
		}
		m.Score = cosineSimilarity(queryEmbedding, embedding)
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			return matches[i].SourceID < matches[j].SourceID
		}
		return matches[i].Score > matches[j].Score
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches, nil
}

func (r *SQLiteVectorRepository) FindSimilarToSource(ctx context.Context, sourceType string, sourceID int64, threshold float64, limit int) ([]VectorMatch, error) {
	if limit <= 0 {
		limit = 10
	}

	// Get the source embedding first
	embedding, err := r.GetEmbedding(ctx, sourceType, sourceID)
	if err != nil {
		return nil, err
	}
	if embedding == nil {
		return nil, nil
	}

	matches, err := r.SearchSimilar(ctx, sourceType, embedding, limit+1)
	if err != nil {
		return nil, err
	}
	filtered := matches[:0]
	for _, m := range matches {
		if m.SourceID == sourceID {
			continue
		}
		if m.Score < threshold {
			continue
		}
		filtered = append(filtered, m)
		if len(filtered) >= limit {
			break
		}
	}
	return filtered, nil
}

func (r *SQLiteVectorRepository) DeleteEmbedding(ctx context.Context, sourceType string, sourceID int64) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM embeddings WHERE source_type = ? AND source_id = ?`,
		sourceType, sourceID,
	)
	return err
}

func (r *SQLiteVectorRepository) HasEmbedding(ctx context.Context, sourceType string, sourceID int64) (bool, error) {
	var exists int
	err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM embeddings WHERE source_type = ? AND source_id = ? AND length(embedding_blob) > 0 LIMIT 1`,
		sourceType, sourceID,
	).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *SQLiteVectorRepository) GetEmbedding(ctx context.Context, sourceType string, sourceID int64) ([]float32, error) {
	var blob []byte
	err := r.db.QueryRowContext(ctx,
		`SELECT embedding_blob FROM embeddings
		 WHERE source_type = ? AND source_id = ? AND length(embedding_blob) > 0
		 ORDER BY created_at DESC, id DESC
		 LIMIT 1`,
		sourceType, sourceID,
	).Scan(&blob)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return deserializeFloat32(blob), nil
}

func serializeFloat32(values []float32) []byte {
	if len(values) == 0 {
		return nil
	}
	buf := make([]byte, len(values)*4)
	for i, value := range values {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(value))
	}
	return buf
}

// deserializeFloat32 converts a raw byte slice (little-endian float32 array) back to []float32.
func deserializeFloat32(b []byte) []float32 {
	if len(b) == 0 || len(b)%4 != 0 {
		return nil
	}
	n := len(b) / 4
	result := make([]float32, n)
	for i := 0; i < n; i++ {
		bits := uint32(b[i*4]) | uint32(b[i*4+1])<<8 | uint32(b[i*4+2])<<16 | uint32(b[i*4+3])<<24
		result[i] = math.Float32frombits(bits)
	}
	return result
}

func cosineSimilarity(left []float32, right []float32) float64 {
	if len(left) == 0 || len(left) != len(right) {
		return 0
	}
	var dot float64
	var leftNorm float64
	var rightNorm float64
	for i := range left {
		l := float64(left[i])
		r := float64(right[i])
		dot += l * r
		leftNorm += l * l
		rightNorm += r * r
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm))
}
