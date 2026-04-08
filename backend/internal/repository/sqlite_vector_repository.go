package repository

import (
	"context"
	"database/sql"
	"errors"
	"math"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

type SQLiteVectorRepository struct {
	db *sql.DB
}

func NewSQLiteVectorRepository(db *sql.DB) *SQLiteVectorRepository {
	return &SQLiteVectorRepository{db: db}
}

func (r *SQLiteVectorRepository) StoreEmbedding(ctx context.Context, sourceType string, sourceID int64, model string, embedding []float32) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Upsert metadata row and get its rowid
	var metaID int64
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM embeddings WHERE source_type = ? AND source_id = ? AND model = ?`,
		sourceType, sourceID, model,
	).Scan(&metaID)

	if errors.Is(err, sql.ErrNoRows) {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO embeddings(source_type, source_id, model, dimensions, created_at)
			 VALUES(?, ?, ?, ?, datetime('now'))`,
			sourceType, sourceID, model, len(embedding),
		)
		if err != nil {
			return err
		}
		metaID, err = res.LastInsertId()
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		// Update existing
		if _, err := tx.ExecContext(ctx,
			`UPDATE embeddings SET dimensions = ?, created_at = datetime('now') WHERE id = ?`,
			len(embedding), metaID,
		); err != nil {
			return err
		}
		// Delete old vec entry so we can re-insert
		if _, err := tx.ExecContext(ctx, `DELETE FROM vec_embeddings WHERE rowid = ?`, metaID); err != nil {
			return err
		}
	}

	// Insert into vec0 virtual table
	serialized, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO vec_embeddings(rowid, embedding) VALUES(?, ?)`,
		metaID, serialized,
	); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *SQLiteVectorRepository) SearchSimilar(ctx context.Context, sourceType string, queryEmbedding []float32, limit int) ([]VectorMatch, error) {
	if limit <= 0 {
		limit = 10
	}

	serialized, err := sqlite_vec.SerializeFloat32(queryEmbedding)
	if err != nil {
		return nil, err
	}

	// Over-fetch because KNN k is applied before the JOIN filter on source_type.
	fetchK := limit * 3
	if fetchK < 50 {
		fetchK = 50
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT e.source_type, e.source_id, v.distance
		 FROM vec_embeddings v
		 JOIN embeddings e ON e.id = v.rowid
		 WHERE v.embedding MATCH ? AND k = ?`,
		serialized, fetchK,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []VectorMatch
	for rows.Next() {
		var m VectorMatch
		var distance float64
		if err := rows.Scan(&m.SourceType, &m.SourceID, &distance); err != nil {
			return nil, err
		}
		if m.SourceType != sourceType {
			continue
		}
		m.Score = 1 - distance
		matches = append(matches, m)
		if len(matches) >= limit {
			break
		}
	}
	return matches, rows.Err()
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

	// Search for similar, fetch extra to filter by threshold
	fetchLimit := limit * 3
	if fetchLimit < 50 {
		fetchLimit = 50
	}

	serialized, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT e.source_type, e.source_id, v.distance
		 FROM vec_embeddings v
		 JOIN embeddings e ON e.id = v.rowid
		 WHERE v.embedding MATCH ? AND k = ?`,
		serialized, fetchLimit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []VectorMatch
	for rows.Next() {
		var m VectorMatch
		var distance float64
		if err := rows.Scan(&m.SourceType, &m.SourceID, &distance); err != nil {
			return nil, err
		}
		m.Score = 1 - distance
		// Skip wrong type, self, and below threshold
		if m.SourceType != sourceType {
			continue
		}
		if m.SourceID == sourceID {
			continue
		}
		if m.Score < threshold {
			continue
		}
		matches = append(matches, m)
		if len(matches) >= limit {
			break
		}
	}
	return matches, rows.Err()
}

func (r *SQLiteVectorRepository) DeleteEmbedding(ctx context.Context, sourceType string, sourceID int64) error {
	var metaID int64
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM embeddings WHERE source_type = ? AND source_id = ?`,
		sourceType, sourceID,
	).Scan(&metaID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM vec_embeddings WHERE rowid = ?`, metaID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM embeddings WHERE id = ?`, metaID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SQLiteVectorRepository) HasEmbedding(ctx context.Context, sourceType string, sourceID int64) (bool, error) {
	var exists int
	err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM embeddings WHERE source_type = ? AND source_id = ? LIMIT 1`,
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
		`SELECT v.embedding FROM vec_embeddings v
		 JOIN embeddings e ON e.id = v.rowid
		 WHERE e.source_type = ? AND e.source_id = ?
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
