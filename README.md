# lowcode-faas

TypeScript Action FaaS with **meta** (control plane) and an embeddable **worker SDK**:

- **Meta** (`cmd/meta`, `:8080`): Action CRUD; on save/update/compile, **TypeScript → JS (esbuild)**; metadata in Postgres; **source + JS always uploaded to S3-compatible OSS**. Exposes `GET /api/actions/{name}/runtime` for workers. Deploy with Docker (see `Dockerfile`) or `make run`.
- **Worker** ([`worker`](./worker) package): **public invoke** embedded in *your* Go service; loads metadata + compiled JS from meta, runs with [fastschema/qjs](https://github.com/fastschema/qjs). Configure via `LOWCODE_FAAS_META_URL` and related env vars. Inject custom Go host functions with `worker.WithHost`.

```
Client → Meta :8080          CRUD / compile → Postgres + S3 OSS
Client → Your service        worker SDK :9090 (example) POST .../invoke
                               ├─ LRU hit? → If-None-Match etag → Meta 304 (no JS body)
                               ├─ miss / changed → Meta /runtime (metadata + js) → cache Put
                               └─ qjs execute → JSON
```

Handler contract:

```ts
import type { ActionRequest, ActionResponse } from 'lowcode-faas/runtime'

export default function handler(req: ActionRequest): ActionResponse {
  return { status: 200, data: { ok: true, query: req.query, body: req.body } }
}
```

Also accepts `export function handler` / `export async function handler`. `host` is a worker global.

Request fields: `context`, `body`, `data`, `query`, `method`, `headers`, `path`.  
Response body (and invoke HTTP JSON): `{ status, data }`.

**Action 技术文档**：[docs/action.md](./docs/action.md)（模型、生命周期、Meta/Worker API、示例）。

## Quick start

```bash
cp .env.example .env
make docker-up          # postgres :5433 + rustfs :9000/:9001

# terminal 1 — meta (or: docker compose --profile meta up -d --build)
make run

# terminal 2 — example worker (embeds SDK; needs META_URL)
make run-worker
```

Embed in your own service:

```go
import "lowcode-faas/worker"

w, err := worker.New(worker.ConfigFromEnv(),
  worker.WithDefaultHost(),
  worker.WithHost(myBinder), // optional custom host.*
)
_ = w.Run(ctx)
// or: mux.Handle("/", w.Handler())
```

See [examples/worker-embed](./examples/worker-embed/) for a full process + custom `host.greet` + editor `host.d.ts` demo.

Playground: `examples/lowcode-faas-playground` (`make faas-playground-dev`).

OSS uses the **S3 API** only (rustfs / MinIO / AWS). Object keys: `action-js/{name}/{etag}.ts` and `.js`.

## Meta API (`:8080`)

| Method | Path | Notes |
|--------|------|-------|
| GET | `/healthz` | `role=meta` |
| GET | `/api/actions` | list (`?group=` / `?q=` optional search) |
| GET | `/api/actions/{name}` | includes TS `content` from OSS |
| GET | `/api/actions/{name}/runtime` | worker: metadata + compiled `js` (supports `If-None-Match` → 304) |
| POST | `/api/actions` | create; empty `content` → default handler template; compiles TS→JS, uploads both to OSS (**201**) |
| PUT | `/api/actions/{id}` | recompile + re-upload on save |
| DELETE | `/api/actions/{id}` | soft delete |
| POST | `/api/actions/{id}/compile` | recompile from OSS source |

## Worker API (SDK listen addr, default `:9090`)

| Method | Path | Notes |
|--------|------|-------|
| GET | `/healthz` | `role=worker` |
| POST | `/api/actions/{name}/invoke` | HTTP-mode request → response body `{ status, data }` (HTTP status = `status`; meta in `X-Faas-*` headers) |

## Runtime

Uses `github.com/fastschema/qjs`. The SDK registers **Go host bindings** on `globalThis.host`. See [examples/host-bindings](./examples/host-bindings/) and [examples/worker-embed](./examples/worker-embed/).

Base types: `js/runtime.d.ts`. Custom `host.*` typings are owned by your frontend (demo: `examples/worker-embed/host.d.ts`).

## Makefile

`make run` · `make run-worker` (example embed) · `make test` · `make tidy` · `make docker-up` · `make migrate` · `make build`

## Env

See `.env.example`. `LOWCODE_FAAS_META_URL` is required by the worker SDK. `LOWCODE_FAAS_JS_CACHE_SIZE` controls the JS LRU (default 128).
