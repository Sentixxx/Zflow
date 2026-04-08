package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

func init() {
	sqlite_vec.Auto()
}

func OpenSQLite(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=ON&_busy_timeout=30000&_synchronous=NORMAL", path)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	// Verify sqlite-vec is loaded
	var vecVersion string
	if err := db.QueryRow("SELECT vec_version()").Scan(&vecVersion); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite-vec not available: %w", err)
	}
	return db, nil
}
