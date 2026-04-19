# lowcode-faas

A lightweight FaaS runtime (**JavaScript only in v1**): user code is **Node ESM** and exports a fixed-signature **`handler`**. The **dispatcher** process exposes an HTTP API and can optionally forward execution to a separate **worker** process. Source can live under local `data/`, or **PostgreSQL + S3** (`db_s3`).

**Languages:** [简体中文](README-zh.md)

## Process roles

| Command | Role |
|---------|------|
| `go run ./cmd/dispatcher` | Public `POST /functions`, `POST /functions/{name}/invoke`; if `LOWCODE_FAAS_WORKER_URL` is set, invoke is proxied to the worker over HTTP |
| `go run ./cmd/worker` | Internal `POST /v1/run`, reads storage and runs user `handler` in **Docker** |

For local dev you may run **dispatcher only** (omit `WORKER_URL`); execution then uses the same logic in-process as the worker.

## User code contract (fixed shape)

- Files are **ESM**; on disk the default filename is `{name}.mjs`.
- **Must** export:

```javascript
/** @param {import('../../js/runtime.d.ts').HandlerInput} input */
export async function handler(input) {
  return { ok: true, data: { message: "..." } };
}
```

- **`input`**: same JSON object as the `input` field on the `invoke` request (any JSON object).
- **Return value** (stored in `RunResult.output`) must be an object with boolean **`ok`**:
  - Success: `{ "ok": true, "data": <anything> }`
  - Business failure: `{ "ok": false, "error": "message", "code": "optional" }`

For TypeScript, see [`js/runtime.d.ts`](js/runtime.d.ts) in this repo.

Execution: the platform writes `input.json` in the container, runs an embedded `bootstrap.mjs` that calls `handler`, then reads `output.json` (user code does **not** open an HTTP port).

## Build

```bash
go build -o dispatcher ./cmd/dispatcher
go build -o worker ./cmd/worker
```

## Configuration and environment variables

Core settings live in [`internal/config/config.go`](internal/config/config.go) and can be overridden via env (see [`internal/config/env.go`](internal/config/env.go)).

| Variable | Meaning |
|----------|---------|
| `LOWCODE_FAAS_LISTEN` | Dispatcher listen address, default `:8080` |
| `LOWCODE_FAAS_WORKER_URL` | If set, invoke is forwarded to this base URL (e.g. `http://127.0.0.1:9090`) |
| `LOWCODE_FAAS_WORKER_LISTEN` | Worker listen address, default `:9090` |
| `LOWCODE_FAAS_STORAGE` | `files` (default) or `db_s3` |
| `LOWCODE_FAAS_POSTGRES_DSN` | PostgreSQL DSN when using `db_s3` |
| `LOWCODE_FAAS_S3_*` | S3 / MinIO settings for `db_s3` |
| `LOWCODE_FAAS_FUNCTION_CACHE` | `true` / `false` |
| `LOWCODE_FAAS_INVOKE_DEFAULT_TIMEOUT_MS` | Default cap in ms when the request omits `timeout_ms` (default `120000`); too low may cause `signal: killed` on first `docker pull` |
| `LOWCODE_FAAS_NODE_DOCKER_IMAGE` | Image for user `handler`, default `node:22-alpine`; use a private registry URL on air-gapped networks |
| `LOWCODE_FAAS_PREWARM_NODE_IMAGE` | Default `true`: background `docker pull` of the image above on startup; set `false` to disable |
| `LOWCODE_FAAS_FUNCTION_LOGS` / `LOWCODE_FAAS_FUNCTION_LOGS_SINK` | Capture user `console.*`. **Recommended:** `docker_json` or `promtail`: one JSON object per line to process stdout → Docker json-file → **Promtail → Loki** (see `docker-compose.dev.yml`). `stdout` keeps the prefixed legacy format. Direct Loki push uses `loki` + `LOWCODE_FAAS_LOG_LOKI_*` (usually avoid duplicating Promtail) |
| `LOWCODE_FAAS_LOGS_GRAFANA_EXPLORE_URL_TEMPLATE` | Optional; `GET /function-logs/query` may include `grafana_explore_url` in JSON, with placeholders `{{logql}}`, `{{run_id}}`, `{{deployment_id}}` (URL-encoded when substituted) |

User code runs via **`docker create` + `docker cp` workspace into the container + `docker start`**, then **`docker cp` out `output.json`**, so the worker does not bind-mount paths that only exist in a sibling container’s `/tmp` (which would cause missing `bootstrap.mjs` / `MODULE_NOT_FOUND`).

**China / Docker registry mirrors (not HTTP proxy):** `docker pull` during `invoke` runs on the **host** Docker daemon (with `docker.sock` mounted). Slow pulls from Docker Hub should be fixed with **registry mirrors**, e.g. Docker Desktop → **Settings → Docker Engine**, add `registry-mirrors` (example: `"registry-mirrors": ["https://docker.m.daocloud.io"]`—use whatever your mirror documents). On Linux, edit **`/etc/docker/daemon.json`** and restart Docker.  
**Without daemon mirror config:** set `LOWCODE_FAAS_NODE_DOCKER_IMAGE` to the **full name** on your mirror (e.g. `docker.m.daocloud.io/library/node:22-alpine`). Keeping `node:22-alpine` uses the default registry; this is unrelated to `HTTP_PROXY`.

## API

### Collections (nested paths)

- `POST /collections` body: `{ "path": "org/team", "env": { ... } }` (`env` optional)—register a path; non-root collections must exist before creating functions under them.
- `GET /collections?prefix=org`—list registered paths (optional prefix filter).
- `PUT /collections/{path}/env` or `PUT /collections/env` (root)—body `{ "env": { ... } }` for collection-level env.
- URL form: `/collections/org/team/functions`, `/collections/org/team/functions/{name}/invoke`, etc. (`org/team` is `collection_path`).

### Create function `POST /functions`

```json
{
  "name": "greet",
  "language": "javascript",
  "source_code": "...",
  "source_url": "https://example.com/handler.mjs",
  "collection_path": "org/team",
  "env": {}
}
```

`collection_path` may be omitted (root collection); or pass `?collection_path=org%2Fteam` on the request.

`language` only supports **`javascript`** (may be omitted; default is `javascript`).

### List functions `GET /functions`

Query `collection_path` selects the collection; omit for root.

### List deployment versions `GET /functions/{name}/versions`

`db_s3` returns full history; `files` returns a single row for the current version only. Same `?collection_path=` or nested URL rules.

### Invoke `POST /functions/{name}/invoke`

```json
{ "input": { "name": "lowcode" }, "timeout_ms": 15000, "env": {}, "version": 3 }
```

Optional **`version`** pins a deployment (`db_s3`: any historical version; `files`: only if it matches current). Also `?version=3`. Collections: `?collection_path=org%2Fteam`.

Response is still `RunResult` (includes `execution_id`, `deployment_version`, `deployment_id`, `collection_path`, etc.); business payload is **`output`** (JSON return value of `handler`).

### Env merge order for invoke

Process env injected into the container is: **ancestor collection env (root → … → current path, child overrides parent) ∪ function env ∪ request body `env`**, with the last winning.

### Execution logs and Loki / Promtail (recommended)

1. Set `LOWCODE_FAAS_FUNCTION_LOGS=true`, `LOWCODE_FAAS_FUNCTION_LOGS_SINK=docker_json` (alias `promtail`).
2. Logs are **one JSON line per event** (`run_id`, `execution_id`, `log_stream`, `function_name`, `collection_path`, `deployment_id`, …) on **worker/dispatcher stdout**, scraped by **Promtail** (`deploy/promtail-dev.yml`) via the Docker API into **Loki**. Local: `docker compose -f docker-compose.dev.yml up` (includes `loki:3100`).
3. **Query helper:** `GET /function-logs/query?run_id=...&function=...&collection_path=...&deployment_id=...` returns JSON with **example queries** per backend (`loki_logql_json`, `loki_logql_substr`, `elasticsearch_query_string`, `aws_cloudwatch_logs_insights`, `google_cloud_logging`). ES, Loki, CloudWatch, and GCP **do not share one query language**; align on **stable JSON field names** and express the same filters in each product.
4. If `LOWCODE_FAAS_LOGS_GRAFANA_EXPLORE_URL_TEMPLATE` is set, the response may include `grafana_explore_url` (template placeholders like `{{logql}}` are substituted server-side).

### Pull update `POST /functions/{name}/pull`

```json
{ "source_url": "https://example.com/handler.mjs" }
```

## Data layout (`files` mode)

- `data/collections/__root__/functions/{name}.mjs` — root collection source (current default)
- `data/functions/{name}.mjs` — **legacy** root source; still read if present when listing/loading root functions
- `data/collections/__root__/functions/{name}.env.json` — root function env (alongside the `.mjs` above)
- `data/functions/{name}.env.json` — env for legacy root `.mjs` only
- `data/collections/{segment}/.../{segment}/functions/{name}.mjs` — non-root collection source
- `data/collections/__root__/collection.env.json` — root collection-level env (optional)
- `data/collections/{path}/collection.env.json` — collection-level env for that path

## systemd

- [`lowcode-faas.service`](lowcode-faas.service): run **dispatcher** (set `ExecStart` to your `dispatcher` binary path).
- If you use a worker: add another unit for `worker`, and set `Environment=LOWCODE_FAAS_WORKER_URL=http://127.0.0.1:9090` on the dispatcher unit.

## Repository layout

```
cmd/dispatcher/main.go   # Dispatcher HTTP entry
cmd/worker/main.go     # Worker HTTP entry
internal/config/       # Config and env overrides
internal/model/        # Types and contracts
internal/store/        # files + PostgreSQL/S3
internal/runner/       # Docker + bootstrap.mjs
internal/httpserver/   # Dispatcher routes
internal/workerserver/ # Worker routes
internal/workerclient/ # Dispatcher → worker HTTP
internal/envutil/ internal/limits/ internal/sourcefetch/
js/runtime.d.ts        # TS types for handlers
```
