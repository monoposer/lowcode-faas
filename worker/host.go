package worker

import (
	"lowcode-faas/internal/runner"
)

// HostBinder registers Go-backed APIs on a QuickJS context before each action invoke.
// Prefer extending globalThis.host (created by WithDefaultHost) via qjs.ToJsValue / ProxyValue.
//
//	worker.WithHost(func(reqCtx context.Context, qctx *qjs.Context, logs *strings.Builder) error {
//		host := qctx.Global().GetPropertyStr("host")
//		fn, err := qjs.ToJsValue(qctx, func(name string) string { return "hi " + name })
//		if err != nil {
//			return err
//		}
//		host.SetPropertyStr("greet", fn)
//		return nil
//	})
type HostBinder = runner.HostBinder

// Option configures a Worker.
type Option func(*options)

type options struct {
	hosts []HostBinder
}

// WithHost appends a host binder (called once per invoke, after earlier binders).
func WithHost(h HostBinder) Option {
	return func(o *options) {
		if h != nil {
			o.hosts = append(o.hosts, h)
		}
	}
}

// WithDefaultHost registers the built-in globalThis.host (log / time / echo / mem / goCtx).
func WithDefaultHost() Option {
	return WithHost(runner.BindDefaultHost)
}
