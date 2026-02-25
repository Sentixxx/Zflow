package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type AppConfig struct {
	Addr            string
	DataDir         string
	DBPath          string
	RefreshInterval time.Duration
	LogLevel        string
	LogFormat       string
}

func Load() AppConfig {
	dataDir := firstNonEmpty(
		os.Getenv("ZFLOW_DATA_DIR"),
		os.Getenv("DATA_DIR"),
		"./data",
	)

	addr := firstNonEmpty(
		os.Getenv("ZFLOW_ADDR"),
		buildAddrFromPort(os.Getenv("PORT")),
		":8080",
	)
	if !strings.HasPrefix(addr, ":") && !strings.Contains(addr, ":") {
		addr = ":" + addr
	}

	dbPath := strings.TrimSpace(os.Getenv("ZFLOW_DB_PATH"))
	if dbPath == "" {
		dbPath = filepath.Join(dataDir, "zflow.db")
	}

	refreshInterval := parseDurationWithFallback(
		firstNonEmpty(os.Getenv("ZFLOW_REFRESH_INTERVAL"), os.Getenv("REFRESH_INTERVAL")),
		15*time.Minute,
	)

	logLevel := firstNonEmpty(os.Getenv("ZFLOW_LOG_LEVEL"), os.Getenv("LOG_LEVEL"), os.Getenv("LOG_INFO_LEVEL"), "info")
	logFormat := firstNonEmpty(os.Getenv("ZFLOW_LOG_FORMAT"), os.Getenv("LOG_FORMAT"), "text")

	return AppConfig{
		Addr:            strings.TrimSpace(addr),
		DataDir:         strings.TrimSpace(dataDir),
		DBPath:          strings.TrimSpace(dbPath),
		RefreshInterval: refreshInterval,
		LogLevel:        strings.ToLower(strings.TrimSpace(logLevel)),
		LogFormat:       strings.ToLower(strings.TrimSpace(logFormat)),
	}
}

func parseDurationWithFallback(raw string, fallback time.Duration) time.Duration {
	text := strings.TrimSpace(raw)
	if text == "" {
		return fallback
	}
	d, err := time.ParseDuration(text)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func buildAddrFromPort(port string) string {
	text := strings.TrimSpace(port)
	if text == "" {
		return ""
	}
	if _, err := strconv.Atoi(text); err != nil {
		return text
	}
	return ":" + text
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
