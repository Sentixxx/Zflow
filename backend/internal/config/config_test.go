package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("ZFLOW_DATA_DIR", "")
	t.Setenv("DATA_DIR", "")
	t.Setenv("ZFLOW_ADDR", "")
	t.Setenv("PORT", "")
	t.Setenv("ZFLOW_DB_PATH", "")
	t.Setenv("ZFLOW_REFRESH_INTERVAL", "")
	t.Setenv("REFRESH_INTERVAL", "")
	t.Setenv("ZFLOW_LOG_LEVEL", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LOG_INFO_LEVEL", "")
	t.Setenv("ZFLOW_LOG_FORMAT", "")
	t.Setenv("LOG_FORMAT", "")

	cfg := Load()
	if cfg.Addr != ":8080" {
		t.Fatalf("Addr = %q, want %q", cfg.Addr, ":8080")
	}
	if cfg.DataDir != "./data" {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, "./data")
	}
	if cfg.DBPath != filepath.Join("./data", "zflow.db") {
		t.Fatalf("DBPath = %q, want %q", cfg.DBPath, filepath.Join("./data", "zflow.db"))
	}
	if cfg.RefreshInterval != 15*time.Minute {
		t.Fatalf("RefreshInterval = %s, want 15m", cfg.RefreshInterval)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.LogFormat != "text" {
		t.Fatalf("LogFormat = %q, want %q", cfg.LogFormat, "text")
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("ZFLOW_DATA_DIR", "/tmp/zflow-data")
	t.Setenv("PORT", "9090")
	t.Setenv("ZFLOW_DB_PATH", "/tmp/custom.db")
	t.Setenv("ZFLOW_REFRESH_INTERVAL", "30s")
	t.Setenv("ZFLOW_LOG_LEVEL", "DEBUG")
	t.Setenv("ZFLOW_LOG_FORMAT", "JSON")

	cfg := Load()
	if cfg.Addr != ":9090" {
		t.Fatalf("Addr = %q, want %q", cfg.Addr, ":9090")
	}
	if cfg.DataDir != "/tmp/zflow-data" {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, "/tmp/zflow-data")
	}
	if cfg.DBPath != "/tmp/custom.db" {
		t.Fatalf("DBPath = %q, want %q", cfg.DBPath, "/tmp/custom.db")
	}
	if cfg.RefreshInterval != 30*time.Second {
		t.Fatalf("RefreshInterval = %s, want 30s", cfg.RefreshInterval)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
	if cfg.LogFormat != "json" {
		t.Fatalf("LogFormat = %q, want %q", cfg.LogFormat, "json")
	}
}
