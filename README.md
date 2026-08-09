# lowcode-faas

TypeScript Action FaaS with **meta** and **runtime worker** split:

- **Meta** (`cmd/meta`, `:8080`): Action CRUD; on save/update/compile, **TypeScript → JS (esbuild)**; metadata in Postgres; **source + JS always uploaded to S3-compatible OSS**. Exposes `GET /api/actions/{name}/runtime` for the worker.
- **Worker** (`cmd/worker`, `:9090`): **public invoke**; loads metadata + compiled JS from meta, runs with [fastschema/qjs](https://github.com/fastschema/qjs).

```
Client → Meta :8080          CRUD / compile → Postgres + S3 OSS
Client → Worker :9090        POST /api/actions/{name}/invoke
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

# terminal 1 — meta
make run

# terminal 2 — worker (needs META_URL)
make run-worker
```

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

## Worker API (`:9090`)

| Method | Path | Notes |
|--------|------|-------|
| GET | `/healthz` | `role=worker` |
| POST | `/api/actions/{name}/invoke` | HTTP-mode request → response body `{ status, data }` (HTTP status = `status`; meta in `X-Faas-*` headers) |

## Runtime

Uses `github.com/fastschema/qjs`. Worker registers **Go host bindings** on `globalThis.host`. See [examples/host-bindings](./examples/host-bindings/).

Types: `js/runtime.d.ts`.

## Makefile

`make run` · `make run-worker` · `make test` · `make tidy` · `make docker-up` · `make migrate`

## Env

See `.env.example`. `LOWCODE_FAAS_META_URL` is required by the worker. `LOWCODE_FAAS_JS_CACHE_SIZE` controls the worker JS LRU (default 128).
