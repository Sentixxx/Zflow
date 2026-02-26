package db

import (
	"path/filepath"
	"testing"
)

func TestOpenSQLiteCreatesParentDirAndConnects(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nested", "zflow.db")
	conn, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer conn.Close()

	var one int
	if err := conn.QueryRow("SELECT 1").Scan(&one); err != nil {
		t.Fatalf("SELECT 1 error = %v", err)
	}
	if one != 1 {
		t.Fatalf("query result = %d, want 1", one)
	}
}
