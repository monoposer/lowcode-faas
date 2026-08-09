# Host bindings example

Worker qjs can call **Go functions** and hold **Go heap objects** via [fastschema/qjs](https://github.com/fastschema/qjs) `SetFunc` / `ToJsValue` / `ProxyValue`.

## What is registered (`globalThis.host`)

| API | Kind | Notes |
|-----|------|--------|
| `host.log(...args)` | `SetFunc` | Appends to invoke `logs` |
| `host.nowMs()` | `ToJsValue` | Go `time.Now().UnixMilli()` |
| `host.echo(v)` | `ToJsValue` | Any → Go → JS round-trip |
| `host.upper(s)` | `ToJsValue` | String helper |
| `host.mem` | **ProxyValue** | Opaque `KVStore` (`*MemoryKV`) — Go heap, not JSON-copied |
| `host.memSet/Get/Len` | `ToJsValue` | Operate on the proxied Go map (params use **interface**, not `*T`) |
| `host.goCtx()` | `SetFunc` + ProxyValue | Request `context.Context` |
| `host.goCtxDeadlineMs(ctx)` | `ToJsValue` | Reads deadline from proxied context |

## Try it

1. Start rustfs + postgres, then worker + meta (`make docker-up`, `make run-worker`, `make run`).
2. In playground, create an action and paste [`handler.ts`](./handler.ts).
3. Save (compiles + uploads to OSS), invoke with:

```json
{ "context": {}, "body": { "key": "greeting", "value": "hello" }, "query": {}, "data": {} }
```

Expected output shape:

```json
{
  "ok": true,
  "upper": "GREETING",
  "echoed": { "key": "greeting", "value": "hello", "at": 0 },
  "fromGoMem": "hello",
  "size": 1,
  "deadlineMs": 0
}
```

`logs` should contain a line from `host.log`.

## Custom Go hosts

In Go:

```go
r := runner.New(
  runner.WithDefaultHost(),
  runner.WithHost(func(reqCtx context.Context, qctx *qjs.Context, logs *strings.Builder) error {
    fn, err := qjs.ToJsValue(qctx, func(name string) string {
      return "hi " + name
    })
    if err != nil {
      return err
    }
    qctx.Global().SetPropertyStr("greet", fn)
    return nil
  }),
)
```

Then JS can call `greet("world")`.
