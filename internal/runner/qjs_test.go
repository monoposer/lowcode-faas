package runner

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestInvokeSmoke(t *testing.T) {
	esm := []byte(`
export default function handler({ ctx, body }) {
  return { ok: true, body: body, n: 1 };
}
`)
	r := New()
	res, err := r.Invoke(context.Background(), esm, map[string]any{"body": map[string]any{"x": 42}}, 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.Error != "" {
		t.Fatalf("invoke error: %s\nlogs: %s", res.Error, res.Logs)
	}
	m, ok := res.Output.(map[string]any)
	if !ok {
		t.Fatalf("output type %T: %#v", res.Output, res.Output)
	}
	if m["ok"] != true {
		t.Fatalf("expected ok: %#v", m)
	}
}

func TestInvokeNamedHandler(t *testing.T) {
	esm := []byte(`
export function handler({ ctx, body }) {
  return { named: true, body: body };
}
`)
	r := New()
	res, err := r.Invoke(context.Background(), esm, map[string]any{"body": map[string]any{"entity": "Order"}}, 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.Error != "" {
		t.Fatalf("invoke error: %s\nlogs: %s", res.Error, res.Logs)
	}
	m, ok := res.Output.(map[string]any)
	if !ok {
		t.Fatalf("output type %T", res.Output)
	}
	if m["named"] != true {
		t.Fatalf("expected named: %#v", m)
	}
}

func TestInvokeAsync(t *testing.T) {
	esm := []byte(`
export default async function handler({ ctx, body }) {
  return { async: true, id: body && body.id };
}
`)
	r := New()
	res, err := r.Invoke(context.Background(), esm, map[string]any{"body": map[string]any{"id": "abc"}}, 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.Error != "" {
		t.Fatalf("invoke error: %s\nlogs: %s", res.Error, res.Logs)
	}
	m, ok := res.Output.(map[string]any)
	if !ok {
		t.Fatalf("output type %T logs=%s", res.Output, res.Logs)
	}
	if m["async"] != true {
		t.Fatalf("expected async: %#v", m)
	}
}

func TestInvokeMissingHandler(t *testing.T) {
	esm := []byte(`export const foo = 1;`)
	r := New()
	res, err := r.Invoke(context.Background(), esm, map[string]any{}, 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.Error == "" {
		t.Fatal("expected error for missing handler")
	}
	if !strings.Contains(res.Error, "handler") {
		t.Fatalf("unexpected error: %s", res.Error)
	}
}

func TestInvokeHostBindings(t *testing.T) {
	esm := []byte(`
export default function handler({ ctx, body }) {
  host.log("hello", body && body.msg);
  const echoed = host.echo({ n: 7 });
  host.memSet(host.mem, "k", "v");
  const got = host.memGet(host.mem, "k");
  const n = host.memLen(host.mem);
  const up = host.upper("ab");
  const ms = host.nowMs();
  const gctx = host.goCtx();
  const dl = host.goCtxDeadlineMs(gctx);
  return { echoed, got, n, up, ms, dl, ctx: ctx ?? null };
}
`)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	r := New(WithDefaultHost())
	res, err := r.Invoke(ctx, esm, map[string]any{"ctx": map[string]any{"req": 1}, "body": map[string]any{"msg": "world"}}, 15*time.Second)

	if err != nil {
		t.Fatal(err)
	}
	if res.Error != "" {
		t.Fatalf("invoke error: %s\nlogs: %s", res.Error, res.Logs)
	}
	if !strings.Contains(res.Logs, "hello") || !strings.Contains(res.Logs, "world") {
		t.Fatalf("expected host.log in logs, got %q", res.Logs)
	}
	m, ok := res.Output.(map[string]any)
	if !ok {
		t.Fatalf("output type %T: %#v", res.Output, res.Output)
	}
	if m["got"] != "v" {
		t.Fatalf("memGet: %#v", m)
	}
	if m["n"] != float64(1) && m["n"] != int(1) && m["n"] != int64(1) && m["n"] != int32(1) {
		t.Fatalf("memLen: %#v", m["n"])
	}
	if m["up"] != "AB" {
		t.Fatalf("upper: %#v", m["up"])
	}
	echoed, _ := m["echoed"].(map[string]any)
	if echoed["n"] != float64(7) && echoed["n"] != int(7) && echoed["n"] != int64(7) {
		t.Fatalf("echo: %#v", m["echoed"])
	}
	if m["ms"] == nil || m["ms"] == float64(0) {
		t.Fatalf("nowMs missing: %#v", m["ms"])
	}
}
