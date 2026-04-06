package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type PostgresVectorRepository struct {
	db *sql.DB
}

func NewPostgresVectorRepository(db *sql.DB) *PostgresVectorRepository {
	return &PostgresVectorRepository{db: db}
}

func (r *PostgresVectorRepository) StoreEmbedding(ctx context.Context, sourceType string, sourceID int64, model string, embedding []float32) error {
	vecLiteral := float32SliceToVectorLiteral(embedding)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO embeddings(source_type, source_id, model, embedding, dimensions)
		 VALUES($1, $2, $3, $4::vector, $5)
		 ON CONFLICT(source_type, source_id, model) DO UPDATE SET
		 embedding = EXCLUDED.embedding,
		 dimensions = EXCLUDED.dimensions,
		 created_at = NOW()`,
		sourceType, sourceID, model, vecLiteral, len(embedding),
	)
	return err
}

func (r *PostgresVectorRepository) SearchSimilar(ctx context.Context, sourceType string, queryEmbedding []float32, limit int) ([]VectorMatch, error) {
	if limit <= 0 {
		limit = 10
	}
	vecLiteral := float32SliceToVectorLiteral(queryEmbedding)
	rows, err := r.db.QueryContext(ctx,
		`SELECT source_type, source_id, 1 - (embedding <=> $1::vector) AS similarity
		 FROM embeddings
		 WHERE source_type = $2
		 ORDER BY embedding <=> $1::vector
		 LIMIT $3`,
		vecLiteral, sourceType, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVectorMatches(rows)
}

func (r *PostgresVectorRepository) FindSimilarToSource(ctx context.Context, sourceType string, sourceID int64, threshold float64, limit int) ([]VectorMatch, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT e2.source_type, e2.source_id, 1 - (e1.embedding <=> e2.embedding) AS similarity
		 FROM embeddings e1
		 JOIN embeddings e2 ON e2.source_type = e1.source_type AND e2.source_id != e1.source_id
		 WHERE e1.source_type = $1 AND e1.source_id = $2
		   AND 1 - (e1.embedding <=> e2.embedding) >= $3
		 ORDER BY e1.embedding <=> e2.embedding
		 LIMIT $4`,
		sourceType, sourceID, threshold, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVectorMatches(rows)
}

func (r *PostgresVectorRepository) DeleteEmbedding(ctx context.Context, sourceType string, sourceID int64) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM embeddings WHERE source_type = $1 AND source_id = $2`,
		sourceType, sourceID,
	)
	return err
}

func (r *PostgresVectorRepository) HasEmbedding(ctx context.Context, sourceType string, sourceID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM embeddings WHERE source_type = $1 AND source_id = $2)`,
		sourceType, sourceID,
	).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return exists, nil
}

func (r *PostgresVectorRepository) GetEmbedding(ctx context.Context, sourceType string, sourceID int64) ([]float32, error) {
	var vecStr string
	err := r.db.QueryRowContext(ctx,
		`SELECT embedding::text FROM embeddings WHERE source_type = $1 AND source_id = $2 LIMIT 1`,
		sourceType, sourceID,
	).Scan(&vecStr)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseVectorLiteral(vecStr), nil
}

func parseVectorLiteral(s string) []float32 {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return nil
	}
	s = s[1 : len(s)-1] // strip [ ]
	parts := strings.Split(s, ",")
	result := make([]float32, 0, len(parts))
	for _, p := range parts {
		var f float64
		if _, err := fmt.Sscanf(strings.TrimSpace(p), "%g", &f); err == nil {
			result = append(result, float32(f))
		}
	}
	return result
}

func scanVectorMatches(rows *sql.Rows) ([]VectorMatch, error) {
	var matches []VectorMatch
	for rows.Next() {
		var m VectorMatch
		if err := rows.Scan(&m.SourceType, &m.SourceID, &m.Score); err != nil {
			return nil, err
		}
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

func float32SliceToVectorLiteral(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%g", f)
	}
	b.WriteByte(']')
	return b.String()
}
