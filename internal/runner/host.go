package runner

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/fastschema/qjs"
)

// HostBinder registers Go-backed APIs on a QuickJS context before the action runs.
// Prefer ProxyValue for in-memory Go objects (zero-copy) and ToJsValue / SetFunc for callables.
type HostBinder func(reqCtx context.Context, qctx *qjs.Context, logs *strings.Builder) error

// Option configures a Runner.
type Option func(*Runner)

// Runner executes compiled action ESM inside QuickJS via fastschema/qjs (wazero).
type Runner struct {
	hosts []HostBinder
}

func New(opts ...Option) *Runner {
	r := &Runner{}
	for _, o := range opts {
		o(r)
	}
	return r
}

// WithHost appends a host binder (called once per invoke).
func WithHost(h HostBinder) Option {
	return func(r *Runner) { r.hosts = append(r.hosts, h) }
}

// WithDefaultHost registers the built-in `host` global (log / time / echo / in-memory KV via ProxyValue).
func WithDefaultHost() Option {
	return WithHost(BindDefaultHost)
}

// MemoryKV is an in-process key/value store shared with JS via ProxyValue (no JSON copy of the map).
type MemoryKV struct {
	mu sync.Mutex
	m  map[string]string
}

func NewMemoryKV() *MemoryKV {
	return &MemoryKV{m: make(map[string]string)}
}

func (s *MemoryKV) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
}

func (s *MemoryKV) Get(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[key]
}

func (s *MemoryKV) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.m)
}

// KVStore is the interface exposed through ProxyValue.
// qjs converts ProxyValue args into interface types reliably (pointer-to-struct params do not).
type KVStore interface {
	Set(key, value string)
	Get(key string) string
	Len() int
}

var _ KVStore = (*MemoryKV)(nil)

// BindDefaultHost exposes:
//
//	host.log(...args)           — append to invoke logs
//	host.nowMs()                — unix millis
//	host.echo(v)                — round-trip any JSON-ish value through Go
//	host.upper(s)               — string helper
//	host.mem                    — ProxyValue(KVStore)  (Go heap, not serialized)
//	host.memSet(mem, key, val)  — write through proxy
//	host.memGet(mem, key)       — read through proxy
//	host.memLen(mem)            — length through proxy
//	host.goCtx()                — ProxyValue(context.Context) for the request
//	host.goCtxDeadlineMs(ctx)   — read deadline from proxied context
func BindDefaultHost(reqCtx context.Context, qctx *qjs.Context, logs *strings.Builder) error {
	if reqCtx == nil {
		reqCtx = context.Background()
	}
	if logs == nil {
		logs = &strings.Builder{}
	}

	host := qctx.NewObject()

	// --- SetFunc style (explicit This/Args) ---
	logFn := qctx.Function(func(this *qjs.This) (*qjs.Value, error) {
		parts := make([]string, 0, len(this.Args()))
		for _, a := range this.Args() {
			parts = append(parts, a.String())
		}
		line := strings.Join(parts, " ")
		logs.WriteString(line)
		logs.WriteByte('\n')
		return qctx.NewUndefined(), nil
	})
	host.SetPropertyStr("log", logFn)

	// --- ToJsValue style (typed Go funcs, args auto-converted) ---
	nowMs, err := qjs.ToJsValue(qctx, func() int64 {
		return time.Now().UnixMilli()
	})
	if err != nil {
		return fmt.Errorf("host.nowMs: %w", err)
	}
	host.SetPropertyStr("nowMs", nowMs)

	echo, err := qjs.ToJsValue(qctx, func(v any) any {
		return v
	})
	if err != nil {
		return fmt.Errorf("host.echo: %w", err)
	}
	host.SetPropertyStr("echo", echo)

	upper, err := qjs.ToJsValue(qctx, func(s string) string {
		return strings.ToUpper(s)
	})
	if err != nil {
		return fmt.Errorf("host.upper: %w", err)
	}
	host.SetPropertyStr("upper", upper)

	// --- ProxyValue: store as interface so JsArgToGo can round-trip the same Go heap object ---
	var mem KVStore = NewMemoryKV()
	host.SetPropertyStr("mem", qctx.NewProxyValue(mem))

	memSet, err := qjs.ToJsValue(qctx, func(m KVStore, key, value string) {
		if m == nil {
			return
		}
		m.Set(key, value)
	})
	if err != nil {
		return fmt.Errorf("host.memSet: %w", err)
	}
	host.SetPropertyStr("memSet", memSet)

	memGet, err := qjs.ToJsValue(qctx, func(m KVStore, key string) string {
		if m == nil {
			return ""
		}
		return m.Get(key)
	})
	if err != nil {
		return fmt.Errorf("host.memGet: %w", err)
	}
	host.SetPropertyStr("memGet", memGet)

	memLen, err := qjs.ToJsValue(qctx, func(m KVStore) int {
		if m == nil {
			return 0
		}
		return m.Len()
	})
	if err != nil {
		return fmt.Errorf("host.memLen: %w", err)
	}
	host.SetPropertyStr("memLen", memLen)

	goCtxFn := qctx.Function(func(this *qjs.This) (*qjs.Value, error) {
		return qctx.NewProxyValue(reqCtx), nil
	})
	host.SetPropertyStr("goCtx", goCtxFn)

	deadlineMs, err := qjs.ToJsValue(qctx, func(c context.Context) int64 {
		if c == nil {
			return 0
		}
		d, ok := c.Deadline()
		if !ok {
			return 0
		}
		return d.UnixMilli()
	})
	if err != nil {
		return fmt.Errorf("host.goCtxDeadlineMs: %w", err)
	}
	host.SetPropertyStr("goCtxDeadlineMs", deadlineMs)

	qctx.Global().SetPropertyStr("host", host)
	return nil
}
