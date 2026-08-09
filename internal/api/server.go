package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Server struct {
	Addr    string
	Handler http.Handler
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.Addr,
		Handler:           s.Handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Printf("lowcode-faas listening on %s", s.Addr)
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
		return fmt.Errorf("listen: %w", err)
	}
}
