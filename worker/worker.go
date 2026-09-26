package worker

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/monoposer/lowcode-faas/internal/api"
	"github.com/monoposer/lowcode-faas/internal/jscache"
	"github.com/monoposer/lowcode-faas/internal/metaclient"
	"github.com/monoposer/lowcode-faas/internal/runner"
)

// Worker is an embeddable FaaS runtime: loads compiled JS from meta and invokes via qjs.
type Worker struct {
	cfg        Config
	handler    http.Handler
	log        *slog.Logger
	meta       *metaclient.Client
	cache      *jscache.LRU
	runnerOpts []runner.Option
}

// New builds a Worker. cfg.MetaURL is required.
// Typical use:
//
//	w, err := worker.New(worker.ConfigFromEnv(), worker.WithDefaultHost(), worker.WithHost(myBinder))
//	if err != nil { ... }
//	_ = w.Run(ctx) // or mux.Handle("/", w.Handler())
//	// or programmatic: w.Invoke(ctx, name, group, input, timeout, extraHosts...)
func New(cfg Config, opts ...Option) (*Worker, error) {
	metaURL := strings.TrimRight(strings.TrimSpace(cfg.MetaURL), "/")
	if metaURL == "" {
		return nil, fmt.Errorf("worker: MetaURL is required (set LOWCODE_FAAS_META_URL)")
	}
	if cfg.Listen == "" {
		cfg.Listen = ":9090"
	}
	if cfg.JSCacheSize < 1 {
		cfg.JSCacheSize = 128
	}

	var o options
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}

	log := cfg.Logger
	if log == nil {
		log = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: parseLogLevel(cfg.LogLevel),
		}))
	}

	runnerOpts := make([]runner.Option, 0, len(o.hosts))
	for _, h := range o.hosts {
		runnerOpts = append(runnerOpts, runner.WithHost(h))
	}
	r := runner.New(runnerOpts...)
	meta := metaclient.New(metaURL)
	cache := jscache.New(cfg.JSCacheSize)
	wh := api.NewWorkerHandler(r, meta, cache, log)

	cfg.MetaURL = metaURL
	return &Worker{
		cfg:        cfg,
		handler:    wh.Routes(),
		log:        log,
		meta:       meta,
		cache:      cache,
		runnerOpts: runnerOpts,
	}, nil
}

// Handler returns the HTTP handler (GET /healthz, POST /api/actions/{name}/invoke).
// Mount under your own mux or pass to http.Server.
func (w *Worker) Handler() http.Handler {
	if w == nil {
		return http.NotFoundHandler()
	}
	return w.handler
}

// ListenAddr returns the configured listen address.
func (w *Worker) ListenAddr() string {
	if w == nil {
		return ""
	}
	return w.cfg.Listen
}

// Run listens on cfg.Listen until ctx is cancelled, then shuts down gracefully.
func (w *Worker) Run(ctx context.Context) error {
	if w == nil {
		return fmt.Errorf("worker: nil")
	}
	srv := &http.Server{
		Addr:              w.cfg.Listen,
		Handler:           w.handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		w.log.Info("worker listening", "addr", w.cfg.Listen, "meta", w.cfg.MetaURL)
		errCh <- srv.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return ctx.Err()
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return fmt.Errorf("worker listen: %w", err)
	}
}
