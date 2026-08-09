# Host bindings example

Worker qjs can call **Go functions** and hold **Go heap objects** via [fastschema/qjs](https://github.com/fastschema/qjs) `SetFunc` / `ToJsValue` / `ProxyValue`.

For a runnable process that embeds the SDK with a custom `host.greet`, see **[../worker-embed](../worker-embed/)**.

## What is registered (`globalThis.host`)

Default host (`worker.WithDefaultHost` / `runner.BindDefaultHost`):

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

1. Start rustfs + postgres, then meta + example worker (`make docker-up`, `make run`, `make run-worker`).
2. In playground, create an action and paste [`handler.ts`](./handler.ts) (or the embed demo [`../worker-embed/handler.ts`](../worker-embed/handler.ts)).
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

Script logs from `host.log` go to worker slog (not the invoke JSON body).

## Custom Go hosts (SDK)

```go
import (
  "context"
  "strings"

  "github.com/fastschema/qjs"
  "lowcode-faas/worker"
)

w, err := worker.New(worker.ConfigFromEnv(),
  worker.WithDefaultHost(),
  worker.WithHost(func(reqCtx context.Context, qctx *qjs.Context, logs *strings.Builder) error {
    host := qctx.Global().GetPropertyStr("host")
    fn, err := qjs.ToJsValue(qctx, func(name string) string {
      return "hi " + name
    })
    if err != nil {
      return err
    }
    host.SetPropertyStr("greet", fn)
    return nil
  }),
)
```

Then JS can call `host.greet("world")`. Keep a matching `host.d.ts` in your frontend (demo: [`../worker-embed/host.d.ts`](../worker-embed/host.d.ts)).
