package logger

import (
	"log/slog"
	"os"
	"strings"
)

type ModuleLogger struct {
	base   *slog.Logger
	module string
}

func NewModuleFromEnv(module string) *ModuleLogger {
	level := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		os.Getenv("ZFLOW_LOG_LEVEL"),
		os.Getenv("LOG_LEVEL"),
		os.Getenv("LOG_INFO_LEVEL"),
	)))
	if level == "" {
		level = "info"
	}
	format := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		os.Getenv("ZFLOW_LOG_FORMAT"),
		os.Getenv("LOG_FORMAT"),
	)))
	if format == "" {
		format = "text"
	}
	return NewModule(module, level, format == "json")
}

func NewModule(module, level string, json bool) *ModuleLogger {
	var lvl slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}
	var handler slog.Handler
	if json {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return &ModuleLogger{
		base:   slog.New(handler),
		module: module,
	}
}

func (l *ModuleLogger) Debug(action, resource, result, msg string, attrs ...any) {
	l.base.Debug(msg, append(l.coreAttrs(action, resource, result), attrs...)...)
}

func (l *ModuleLogger) Info(action, resource, result, msg string, attrs ...any) {
	l.base.Info(msg, append(l.coreAttrs(action, resource, result), attrs...)...)
}

func (l *ModuleLogger) Warn(action, resource, result, msg string, attrs ...any) {
	l.base.Warn(msg, append(l.coreAttrs(action, resource, result), attrs...)...)
}

func (l *ModuleLogger) Error(action, resource, result, msg string, attrs ...any) {
	l.base.Error(msg, append(l.coreAttrs(action, resource, result), attrs...)...)
}

func (l *ModuleLogger) coreAttrs(action, resource, result string) []any {
	return []any{
		"module", sanitizeValue(l.module),
		"action", sanitizeValue(action),
		"resource", sanitizeValue(resource),
		"result", sanitizeValue(result),
	}
}

func sanitizeValue(v string) string {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return "unknown"
	}
	return trimmed
}

func ExtractHost(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "://")
	hostPart := trimmed
	if len(parts) == 2 {
		hostPart = parts[1]
	}
	hostPart = strings.Split(hostPart, "/")[0]
	hostPart = strings.Split(hostPart, "?")[0]
	return strings.TrimSpace(hostPart)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
