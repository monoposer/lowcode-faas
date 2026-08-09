# Worker embed example

Embed the [lowcode-faas/worker](../../worker) SDK in your own Go process. Meta stays a separate service; this process only runs invoke + your custom `host` bindings.

## Run

```bash
# terminal 1 — meta (CRUD / compile)
make run

# terminal 2 — this example worker
make run-worker
# or: go run ./examples/worker-embed
```

Env (see [`.env.example`](../../.env.example)):

| Variable | Default | Notes |
|----------|---------|--------|
| `LOWCODE_FAAS_META_URL` | `http://127.0.0.1:8080` | Meta base URL |
| `LOWCODE_FAAS_WORKER_LISTEN` | `:9090` | Invoke listen addr |
| `LOWCODE_FAAS_JS_CACHE_SIZE` | `128` | Compiled JS LRU |
| `LOWCODE_FAAS_LOG_LEVEL` | `info` | Worker slog level |

## Custom host (Go)

[`main.go`](./main.go) registers the default `host` plus `host.greet`:

```go
w, err := worker.New(worker.ConfigFromEnv(),
  worker.WithDefaultHost(),
  worker.WithHost(bindGreetHost),
)
```

Mount under your own mux instead of `Run`:

```go
mux.Handle("/", w.Handler()) // GET /healthz, POST /api/actions/{name}/invoke
```

## Action + editor types

1. Create an action in Meta / playground and paste [`handler.ts`](./handler.ts).
2. Invoke with `{ "body": { "name": "Ada" } }` — expect `data.greeting` = `"hello, Ada"`.

**Frontend types are yours.** Base types live in [`js/runtime.d.ts`](../../js/runtime.d.ts). Extend them with a file like [`host.d.ts`](./host.d.ts) and load it in Monaco:

```ts
monaco.languages.typescript.typescriptDefaults.addExtraLib(
  hostDtsSource,
  'file:///node_modules/lowcode-faas/host.d.ts',
)
```

Keep Go `WithHost` and your `.d.ts` in sync; Meta does not store or serve host typings.
