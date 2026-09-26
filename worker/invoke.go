package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/monoposer/lowcode-faas/internal/jscache"
	"github.com/monoposer/lowcode-faas/internal/metaclient"
	"github.com/monoposer/lowcode-faas/internal/runner"
)

// InvokeResult is the outcome of a programmatic action invoke.
type InvokeResult struct {
	Output   any
	Logs     string
	Duration time.Duration
	Error    string
	Etag     string
	Name     string
}

// Invoke loads compiled JS from meta (with LRU + If-None-Match) and runs it via qjs.
// extraHosts are bound after the Worker-level hosts for this call only (e.g. platform host APIs).
func (w *Worker) Invoke(ctx context.Context, name, group string, input any, timeout time.Duration, extraHosts ...HostBinder) (*InvokeResult, error) {
	if w == nil {
		return nil, fmt.Errorf("worker: nil")
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("worker: action name is required")
	}
	if strings.TrimSpace(group) == "" {
		return nil, fmt.Errorf("worker: group is required")
	}

	rt, err := w.resolveRuntime(ctx, name, group)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = time.Duration(rt.Timeout) * time.Second
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	opts := append([]runner.Option{}, w.runnerOpts...)
	for _, h := range extraHosts {
		if h != nil {
			opts = append(opts, runner.WithHost(h))
		}
	}
	r := runner.New(opts...)
	res, err := r.Invoke(ctx, []byte(rt.JS), input, timeout)
	if err != nil {
		return nil, err
	}
	out := &InvokeResult{
		Output:   res.Output,
		Logs:     res.Logs,
		Duration: res.Duration,
		Error:    res.Error,
		Etag:     rt.Etag,
		Name:     rt.Name,
	}
	if res.Error != "" {
		return out, fmt.Errorf("%s", res.Error)
	}
	return out, nil
}

func (w *Worker) resolveRuntime(ctx context.Context, name, group string) (*metaclient.RuntimeAction, error) {
	key := jscache.Key(group, name)
	cached, hit := w.cache.Get(key)
	ifNoneMatch := ""
	if hit {
		ifNoneMatch = cached.Etag
	}

	rt, notModified, err := w.meta.GetRuntime(ctx, name, group, ifNoneMatch)
	if err != nil {
		return nil, err
	}
	if notModified {
		if !hit {
			return nil, errors.New("meta returned 304 but cache miss")
		}
		return &metaclient.RuntimeAction{
			Name:    cached.Name,
			Group:   cached.Group,
			Etag:    cached.Etag,
			Timeout: cached.Timeout,
			JsURL:   cached.JsURL,
			JS:      cached.JS,
		}, nil
	}

	w.cache.Put(key, jscache.Entry{
		Name:    rt.Name,
		Group:   rt.Group,
		Etag:    rt.Etag,
		Timeout: rt.Timeout,
		JsURL:   rt.JsURL,
		JS:      rt.JS,
	})
	return rt, nil
}
