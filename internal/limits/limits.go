package limits

import (
	"context"

	"lowcode-faas/internal/config"
)

var runConcurrency chan struct{}

func Init() {
	if config.MaxConcurrentFunctionRuns <= 0 {
		runConcurrency = nil
		return
	}
	runConcurrency = make(chan struct{}, config.MaxConcurrentFunctionRuns)
}

func AcquireRun(ctx context.Context) error {
	if runConcurrency == nil {
		return nil
	}
	select {
	case runConcurrency <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func ReleaseRun() {
	if runConcurrency == nil {
		return
	}
	<-runConcurrency
}
