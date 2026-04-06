package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sentixxx/Zflow/backend/internal/model"
	"github.com/jackc/pgx/v5/pgtype"
)

type PostgresAgentRepository struct {
	db *sql.DB
}

func NewPostgresAgentRepository(db *sql.DB) *PostgresAgentRepository {
	return &PostgresAgentRepository{db: db}
}

// --- Agent Runs ---

func (r *PostgresAgentRepository) CreateAgentRun(ctx context.Context, agentType string, inputSummary string) (model.AgentRun, error) {
	now := time.Now().UTC()
	var id int64
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO agent_runs(agent_type, status, input_summary, started_at) VALUES($1, 'running', $2, $3) RETURNING id`,
		agentType, inputSummary, now,
	).Scan(&id)
	if err != nil {
		return model.AgentRun{}, err
	}
	return model.AgentRun{ID: id, AgentType: agentType, Status: "running", InputSummary: inputSummary, StartedAt: now}, nil
}

func (r *PostgresAgentRepository) CompleteAgentRun(ctx context.Context, id int64, outputSummary string, itemsProcessed, itemsCreated int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE agent_runs SET status = 'completed', output_summary = $1, items_processed = $2, items_created = $3, completed_at = $4 WHERE id = $5`,
		outputSummary, itemsProcessed, itemsCreated, time.Now().UTC(), id,
	)
	return err
}

func (r *PostgresAgentRepository) FailAgentRun(ctx context.Context, id int64, errMsg string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE agent_runs SET status = 'failed', error = $1, completed_at = $2 WHERE id = $3`,
		errMsg, time.Now().UTC(), id,
	)
	return err
}

func (r *PostgresAgentRepository) ListAgentRuns(ctx context.Context, agentType string, limit int) ([]model.AgentRun, error) {
	if limit <= 0 {
		limit = 20
	}
	query := `SELECT id, agent_type, status, input_summary, output_summary, items_processed, items_created, error, started_at, completed_at FROM agent_runs`
	args := []any{}
	argIdx := 1
	if agentType != "" {
		query += fmt.Sprintf(` WHERE agent_type = $%d`, argIdx)
		args = append(args, agentType)
		argIdx++
	}
	query += fmt.Sprintf(` ORDER BY id DESC LIMIT $%d`, argIdx)
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []model.AgentRun
	for rows.Next() {
		var run model.AgentRun
		var completedAt sql.NullTime
		if err := rows.Scan(&run.ID, &run.AgentType, &run.Status, &run.InputSummary, &run.OutputSummary,
			&run.ItemsProcessed, &run.ItemsCreated, &run.Error, &run.StartedAt, &completedAt); err != nil {
			return nil, err
		}
		if completedAt.Valid {
			run.CompletedAt = &completedAt.Time
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// --- Interest Profiles ---

func (r *PostgresAgentRepository) ListInterestProfiles(ctx context.Context) ([]model.InterestProfile, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, label, weight, source_article_ids, last_reinforced_at, created_at, updated_at FROM user_interest_profiles ORDER BY weight DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanInterestProfiles(rows)
}

func (r *PostgresAgentRepository) GetInterestProfile(ctx context.Context, id int64) (model.InterestProfile, bool, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, label, weight, source_article_ids, last_reinforced_at, created_at, updated_at FROM user_interest_profiles WHERE id = $1`, id)
	p, err := scanInterestProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.InterestProfile{}, false, nil
	}
	if err != nil {
		return model.InterestProfile{}, false, err
	}
	return p, true, nil
}

func (r *PostgresAgentRepository) GetInterestProfileEmbedding(ctx context.Context, id int64) ([]float32, error) {
	var vecStr string
	err := r.db.QueryRowContext(ctx,
		`SELECT interest_embedding::text FROM user_interest_profiles WHERE id = $1`, id,
	).Scan(&vecStr)
	if err != nil {
		return nil, err
	}
	return parseVectorLiteral(vecStr), nil
}

func (r *PostgresAgentRepository) CreateInterestProfile(ctx context.Context, label string, embedding []float32, weight float64, articleIDs []int64) (model.InterestProfile, error) {
	now := time.Now().UTC()
	vecLiteral := float32SliceToVectorLiteral(embedding)
	var id int64
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO user_interest_profiles(label, interest_embedding, weight, source_article_ids, last_reinforced_at, created_at, updated_at)
		 VALUES($1, $2::vector, $3, $4, $5, $6, $7) RETURNING id`,
		label, vecLiteral, weight, pgInt64Array(articleIDs), now, now, now,
	).Scan(&id)
	if err != nil {
		return model.InterestProfile{}, err
	}
	return model.InterestProfile{ID: id, Label: label, Weight: weight, SourceArticleIDs: articleIDs, LastReinforcedAt: &now, CreatedAt: now, UpdatedAt: now}, nil
}

func (r *PostgresAgentRepository) UpdateInterestProfileEmbedding(ctx context.Context, id int64, embedding []float32, weight float64, articleIDs []int64) error {
	now := time.Now().UTC()
	vecLiteral := float32SliceToVectorLiteral(embedding)
	_, err := r.db.ExecContext(ctx,
		`UPDATE user_interest_profiles SET interest_embedding = $1::vector, weight = $2, source_article_ids = $3, last_reinforced_at = $4, updated_at = $5 WHERE id = $6`,
		vecLiteral, weight, pgInt64Array(articleIDs), now, now, id,
	)
	return err
}

func (r *PostgresAgentRepository) DeleteInterestProfile(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM user_interest_profiles WHERE id = $1`, id)
	return err
}

func (r *PostgresAgentRepository) FindMatchingInterestProfile(ctx context.Context, embedding []float32, threshold float64) (model.InterestProfile, bool, error) {
	vecLiteral := float32SliceToVectorLiteral(embedding)
	row := r.db.QueryRowContext(ctx,
		`SELECT id, label, weight, source_article_ids, last_reinforced_at, created_at, updated_at,
		        1 - (interest_embedding <=> $1::vector) AS similarity
		 FROM user_interest_profiles
		 WHERE 1 - (interest_embedding <=> $1::vector) >= $2
		 ORDER BY interest_embedding <=> $1::vector
		 LIMIT 1`,
		vecLiteral, threshold,
	)
	var p model.InterestProfile
	var similarity float64
	var articleIDs pgtype.FlatArray[int64]
	var lastReinforced sql.NullTime
	err := row.Scan(&p.ID, &p.Label, &p.Weight, &articleIDs, &lastReinforced, &p.CreatedAt, &p.UpdatedAt, &similarity)
	if errors.Is(err, sql.ErrNoRows) {
		return model.InterestProfile{}, false, nil
	}
	if err != nil {
		return model.InterestProfile{}, false, err
	}
	p.SourceArticleIDs = articleIDs
	if lastReinforced.Valid {
		p.LastReinforcedAt = &lastReinforced.Time
	}
	return p, true, nil
}

func (r *PostgresAgentRepository) ScoreArticleAgainstInterests(ctx context.Context, articleEmbedding []float32) (float64, error) {
	vecLiteral := float32SliceToVectorLiteral(articleEmbedding)
	var score sql.NullFloat64
	err := r.db.QueryRowContext(ctx,
		`SELECT MAX((1 - (interest_embedding <=> $1::vector)) * weight)
		 FROM user_interest_profiles
		 WHERE weight > 0`,
		vecLiteral,
	).Scan(&score)
	if err != nil {
		return 0, err
	}
	if !score.Valid {
		return 0, nil
	}
	return score.Float64, nil
}

// --- Topic Clusters ---

func (r *PostgresAgentRepository) ListActiveClusters(ctx context.Context) ([]model.TopicCluster, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, title, summary, article_count, status, created_at, updated_at FROM topic_clusters WHERE status = 'active' ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clusters []model.TopicCluster
	for rows.Next() {
		var c model.TopicCluster
		if err := rows.Scan(&c.ID, &c.Title, &c.Summary, &c.ArticleCount, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		clusters = append(clusters, c)
	}
	return clusters, rows.Err()
}

func (r *PostgresAgentRepository) GetCluster(ctx context.Context, id int64) (model.TopicCluster, bool, error) {
	var c model.TopicCluster
	err := r.db.QueryRowContext(ctx,
		`SELECT id, title, summary, article_count, status, created_at, updated_at FROM topic_clusters WHERE id = $1`, id,
	).Scan(&c.ID, &c.Title, &c.Summary, &c.ArticleCount, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.TopicCluster{}, false, nil
	}
	if err != nil {
		return model.TopicCluster{}, false, err
	}
	return c, true, nil
}

func (r *PostgresAgentRepository) CreateCluster(ctx context.Context, title string, centroidEmbedding []float32) (model.TopicCluster, error) {
	now := time.Now().UTC()
	vecLiteral := float32SliceToVectorLiteral(centroidEmbedding)
	var id int64
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO topic_clusters(title, centroid_embedding, article_count, status, created_at, updated_at)
		 VALUES($1, $2::vector, 0, 'active', $3, $4) RETURNING id`,
		title, vecLiteral, now, now,
	).Scan(&id)
	if err != nil {
		return model.TopicCluster{}, err
	}
	return model.TopicCluster{ID: id, Title: title, ArticleCount: 0, Status: "active", CreatedAt: now, UpdatedAt: now}, nil
}

func (r *PostgresAgentRepository) UpdateClusterCentroid(ctx context.Context, id int64, centroidEmbedding []float32, articleCount int) error {
	vecLiteral := float32SliceToVectorLiteral(centroidEmbedding)
	_, err := r.db.ExecContext(ctx,
		`UPDATE topic_clusters SET centroid_embedding = $1::vector, article_count = $2, updated_at = $3 WHERE id = $4`,
		vecLiteral, articleCount, time.Now().UTC(), id,
	)
	return err
}

func (r *PostgresAgentRepository) UpdateClusterSummary(ctx context.Context, id int64, title, summary string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE topic_clusters SET title = $1, summary = $2, updated_at = $3 WHERE id = $4`,
		title, summary, time.Now().UTC(), id,
	)
	return err
}

func (r *PostgresAgentRepository) ArchiveCluster(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE topic_clusters SET status = 'archived', updated_at = $1 WHERE id = $2`,
		time.Now().UTC(), id,
	)
	return err
}

func (r *PostgresAgentRepository) AddClusterMember(ctx context.Context, clusterID, articleID int64, similarity float64, isRepresentative bool) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO topic_cluster_members(cluster_id, article_id, similarity, is_representative, added_at)
		 VALUES($1, $2, $3, $4, $5)
		 ON CONFLICT(cluster_id, article_id) DO UPDATE SET similarity = EXCLUDED.similarity, is_representative = EXCLUDED.is_representative`,
		clusterID, articleID, similarity, isRepresentative, time.Now().UTC(),
	)
	if err == nil {
		_, _ = r.db.ExecContext(ctx,
			`UPDATE topic_clusters SET article_count = (SELECT COUNT(*) FROM topic_cluster_members WHERE cluster_id = $1), updated_at = $2 WHERE id = $1`,
			clusterID, time.Now().UTC(),
		)
	}
	return err
}

func (r *PostgresAgentRepository) ListClusterMembers(ctx context.Context, clusterID int64) ([]model.TopicClusterMember, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT cluster_id, article_id, similarity, is_representative, added_at FROM topic_cluster_members WHERE cluster_id = $1 ORDER BY similarity DESC`, clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []model.TopicClusterMember
	for rows.Next() {
		var m model.TopicClusterMember
		if err := rows.Scan(&m.ClusterID, &m.ArticleID, &m.Similarity, &m.IsRepresentative, &m.AddedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *PostgresAgentRepository) GetArticleCluster(ctx context.Context, articleID int64) (*model.TopicCluster, error) {
	var c model.TopicCluster
	err := r.db.QueryRowContext(ctx,
		`SELECT tc.id, tc.title, tc.summary, tc.article_count, tc.status, tc.created_at, tc.updated_at
		 FROM topic_clusters tc
		 JOIN topic_cluster_members tcm ON tcm.cluster_id = tc.id
		 WHERE tcm.article_id = $1 AND tc.status = 'active'
		 LIMIT 1`, articleID,
	).Scan(&c.ID, &c.Title, &c.Summary, &c.ArticleCount, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *PostgresAgentRepository) FindMatchingCluster(ctx context.Context, embedding []float32, threshold float64) (model.TopicCluster, bool, error) {
	vecLiteral := float32SliceToVectorLiteral(embedding)
	var c model.TopicCluster
	err := r.db.QueryRowContext(ctx,
		`SELECT id, title, summary, article_count, status, created_at, updated_at
		 FROM topic_clusters
		 WHERE status = 'active' AND 1 - (centroid_embedding <=> $1::vector) >= $2
		 ORDER BY centroid_embedding <=> $1::vector
		 LIMIT 1`,
		vecLiteral, threshold,
	).Scan(&c.ID, &c.Title, &c.Summary, &c.ArticleCount, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.TopicCluster{}, false, nil
	}
	if err != nil {
		return model.TopicCluster{}, false, err
	}
	return c, true, nil
}

func (r *PostgresAgentRepository) SetRepresentativeArticle(ctx context.Context, clusterID, articleID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE topic_cluster_members SET is_representative = FALSE WHERE cluster_id = $1`, clusterID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE topic_cluster_members SET is_representative = TRUE WHERE cluster_id = $1 AND article_id = $2`, clusterID, articleID); err != nil {
		return err
	}
	return tx.Commit()
}

// --- Topic Briefs ---

func (r *PostgresAgentRepository) CreateTopicBrief(ctx context.Context, brief model.TopicBrief) (model.TopicBrief, error) {
	now := time.Now().UTC()
	var id int64
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO topic_briefs(title, slug, content, level, period_start, period_end, source_cluster_ids, source_article_ids, parent_brief_id, created_at, updated_at)
		 VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id`,
		brief.Title, brief.Slug, brief.Content, brief.Level, brief.PeriodStart, brief.PeriodEnd,
		pgInt64Array(brief.SourceClusterIDs), pgInt64Array(brief.SourceArticleIDs),
		pgNullableBigint(brief.ParentBriefID), now, now,
	).Scan(&id)
	if err != nil {
		return model.TopicBrief{}, err
	}
	brief.ID = id
	brief.CreatedAt = now
	brief.UpdatedAt = now
	return brief, nil
}

func (r *PostgresAgentRepository) GetTopicBrief(ctx context.Context, id int64) (model.TopicBrief, bool, error) {
	var b model.TopicBrief
	var clusterIDs, articleIDs pgtype.FlatArray[int64]
	var parentID sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		`SELECT id, title, slug, content, level, period_start, period_end, source_cluster_ids, source_article_ids, parent_brief_id, created_at, updated_at
		 FROM topic_briefs WHERE id = $1`, id,
	).Scan(&b.ID, &b.Title, &b.Slug, &b.Content, &b.Level, &b.PeriodStart, &b.PeriodEnd,
		&clusterIDs, &articleIDs, &parentID, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.TopicBrief{}, false, nil
	}
	if err != nil {
		return model.TopicBrief{}, false, err
	}
	b.SourceClusterIDs = clusterIDs
	b.SourceArticleIDs = articleIDs
	if parentID.Valid {
		b.ParentBriefID = &parentID.Int64
	}
	return b, true, nil
}

func (r *PostgresAgentRepository) ListTopicBriefs(ctx context.Context, level string, limit int) ([]model.TopicBrief, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, title, slug, content, level, period_start, period_end, source_cluster_ids, source_article_ids, parent_brief_id, created_at, updated_at
		 FROM topic_briefs WHERE level = $1 ORDER BY period_start DESC LIMIT $2`, level, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var briefs []model.TopicBrief
	for rows.Next() {
		var b model.TopicBrief
		var clusterIDs, articleIDs pgtype.FlatArray[int64]
		var parentID sql.NullInt64
		if err := rows.Scan(&b.ID, &b.Title, &b.Slug, &b.Content, &b.Level, &b.PeriodStart, &b.PeriodEnd,
			&clusterIDs, &articleIDs, &parentID, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		b.SourceClusterIDs = clusterIDs
		b.SourceArticleIDs = articleIDs
		if parentID.Valid {
			b.ParentBriefID = &parentID.Int64
		}
		briefs = append(briefs, b)
	}
	return briefs, rows.Err()
}

func (r *PostgresAgentRepository) GetLatestBrief(ctx context.Context, level string) (model.TopicBrief, bool, error) {
	briefs, err := r.ListTopicBriefs(ctx, level, 1)
	if err != nil {
		return model.TopicBrief{}, false, err
	}
	if len(briefs) == 0 {
		return model.TopicBrief{}, false, nil
	}
	return briefs[0], true, nil
}

// --- Helpers ---

func scanInterestProfiles(rows *sql.Rows) ([]model.InterestProfile, error) {
	var profiles []model.InterestProfile
	for rows.Next() {
		var p model.InterestProfile
		var articleIDs pgtype.FlatArray[int64]
		var lastReinforced sql.NullTime
		if err := rows.Scan(&p.ID, &p.Label, &p.Weight, &articleIDs, &lastReinforced, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.SourceArticleIDs = articleIDs
		if lastReinforced.Valid {
			p.LastReinforcedAt = &lastReinforced.Time
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

func scanInterestProfile(scanner interface{ Scan(dest ...any) error }) (model.InterestProfile, error) {
	var p model.InterestProfile
	var articleIDs pgtype.FlatArray[int64]
	var lastReinforced sql.NullTime
	err := scanner.Scan(&p.ID, &p.Label, &p.Weight, &articleIDs, &lastReinforced, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return model.InterestProfile{}, err
	}
	p.SourceArticleIDs = articleIDs
	if lastReinforced.Valid {
		p.LastReinforcedAt = &lastReinforced.Time
	}
	return p, nil
}

func pgInt64Array(ids []int64) string {
	if len(ids) == 0 {
		return "{}"
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprint(id)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func pgNullableBigint(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}
