package repository

import (
	"context"

	"github.com/Sentixxx/Zflow/backend/internal/db"
)

// NewTestSQLiteFeedRepository opens a SQLite database at the given path,
// runs migrations, and returns a ready-to-use repository.
// This is intended for use in tests across packages.
func NewTestSQLiteFeedRepository(dbPath string) (*SQLiteFeedRepository, error) {
	conn, err := db.OpenSQLite(dbPath)
	if err != nil {
		return nil, err
	}
	if err := db.RunMigrations(context.Background(), conn); err != nil {
		conn.Close()
		return nil, err
	}
	return NewSQLiteFeedRepository(conn), nil
}
