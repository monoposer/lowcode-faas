package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/monoposer/lowcode-faas/internal/api"
	"github.com/monoposer/lowcode-faas/internal/config"
	"github.com/monoposer/lowcode-faas/internal/store"
	"github.com/monoposer/lowcode-faas/internal/tscompile"
)

// Meta process: Action CRUD + TS→JS compile on save. Artifacts always go to S3 OSS.
// Exposes GET /api/actions/{name}/runtime for the worker. Public invoke is on the worker.
func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := store.NewPostgres(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()

	uploader, err := store.NewUploaderFromConfig(ctx, cfg)
	if err != nil {
		log.Fatalf("oss (s3): %v", err)
	}

	compiler := tscompile.New(tscompile.Options{Mock: cfg.JSMock})
	h := api.NewHandler(db, compiler, uploader)
	srv := &api.Server{Addr: cfg.Listen, Handler: h.Routes()}
	log.Printf("meta listening on %s (oss=%s/%s)", cfg.Listen, cfg.S3Endpoint, cfg.S3Bucket)
	if err := srv.ListenAndServe(ctx); err != nil && err != context.Canceled {
		log.Fatalf("meta: %v", err)
	}
}
