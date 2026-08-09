package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"lowcode-faas/internal/api"
	"lowcode-faas/internal/config"
	"lowcode-faas/internal/jscache"
	"lowcode-faas/internal/metaclient"
	"lowcode-faas/internal/runner"
)

// Worker serves public invoke. It loads metadata + compiled JS from meta (LRU + etag), then runs qjs.
func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if cfg.MetaURL == "" {
		log.Fatal("LOWCODE_FAAS_META_URL is required")
	}

	logger := newLogger()
	r := runner.New(runner.WithDefaultHost())
	meta := metaclient.New(cfg.MetaURL)
	cache := jscache.New(cfg.JSCacheSize)
	h := api.NewWorkerHandler(r, meta, cache, logger)
	srv := &api.Server{Addr: cfg.WorkerListen, Handler: h.Routes()}
	logger.Info("worker listening",
		"addr", cfg.WorkerListen,
		"meta", cfg.MetaURL,
		"cache_size", cfg.JSCacheSize,
		"host", "default",
	)
	if err := srv.ListenAndServe(ctx); err != nil && err != context.Canceled {
		logger.Error("worker stopped", "error", err.Error())
		os.Exit(1)
	}
}

func newLogger() *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOWCODE_FAAS_LOG_LEVEL"))) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(h).With("service", "lowcode-faas-worker")
}
