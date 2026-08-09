package worker

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Config configures an embedded worker (meta URL, listen addr, cache, logging).
type Config struct {
	// MetaURL is the meta service base URL (required), e.g. http://127.0.0.1:8080.
	// Env: LOWCODE_FAAS_META_URL
	MetaURL string

	// Listen is the HTTP listen address for invoke + healthz.
	// Env: LOWCODE_FAAS_WORKER_LISTEN (default ":9090")
	Listen string

	// JSCacheSize is the in-memory LRU capacity for compiled JS.
	// Env: LOWCODE_FAAS_JS_CACHE_SIZE (default 128)
	JSCacheSize int

	// LogLevel sets slog level for invoke logs: debug|info|warn|error.
	// Env: LOWCODE_FAAS_LOG_LEVEL (default "info")
	LogLevel string

	// Logger overrides the default slog logger. When nil, a JSON handler at LogLevel is used.
	Logger *slog.Logger
}

// ConfigFromEnv loads Config from LOWCODE_FAAS_* environment variables.
func ConfigFromEnv() Config {
	return Config{
		MetaURL:     strings.TrimRight(env("LOWCODE_FAAS_META_URL", "http://127.0.0.1:8080"), "/"),
		Listen:      env("LOWCODE_FAAS_WORKER_LISTEN", ":9090"),
		JSCacheSize: envInt("LOWCODE_FAAS_JS_CACHE_SIZE", 128),
		LogLevel:    env("LOWCODE_FAAS_LOG_LEVEL", "info"),
	}
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
