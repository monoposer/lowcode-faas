// Example: embed lowcode-faas worker SDK in your own process.
//
//	make run          # meta :8080
//	make run-worker   # this example :9090
package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"strings"
	"syscall"

	"github.com/fastschema/qjs"

	"lowcode-faas/worker"
)

func main() {
	cfg := worker.ConfigFromEnv()
	w, err := worker.New(cfg,
		worker.WithDefaultHost(),
		worker.WithHost(bindGreetHost),
	)
	if err != nil {
		log.Fatalf("worker: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("example worker-embed listening on %s (meta=%s)", w.ListenAddr(), cfg.MetaURL)
	if err := w.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("worker: %v", err)
	}
}

// bindGreetHost extends globalThis.host with a customer function (demo).
// Keep host.d.ts in sync for your editor IntelliSense.
func bindGreetHost(reqCtx context.Context, qctx *qjs.Context, logs *strings.Builder) error {
	_ = reqCtx
	_ = logs

	host := qctx.Global().GetPropertyStr("host")
	if host == nil || !host.IsObject() {
		return fmt.Errorf("host global missing; register worker.WithDefaultHost() first")
	}

	greet, err := qjs.ToJsValue(qctx, func(name string) string {
		if name == "" {
			name = "world"
		}
		return "hello, " + name
	})
	if err != nil {
		return fmt.Errorf("host.greet: %w", err)
	}
	host.SetPropertyStr("greet", greet)
	return nil
}
