# Action 技术文档

本文描述 **lowcode-faas** 中 Action 的数据模型、生命周期、Handler 约定、Meta/Worker API，以及存储与运行时行为。

## 1. 概念

Action 是一段可部署的 **TypeScript 函数**：

- **元数据**存 Postgres（名称、分组、版本、超时、OSS URL、etag 等）
- **源码（`.ts`）与编译产物（`.js`）**存 S3 兼容 OSS
- **Meta**（`:8080`）负责 CRUD、TS→JS 编译、对 Worker 提供 runtime 拉取（Docker / `cmd/meta`）
- **Worker**（[`worker`](../worker) SDK，默认 `:9090`）嵌入客户进程，负责对外 **invoke**，用 qjs 执行已编译 JS；示例进程见 `examples/worker-embed`

```
Studio / Playground / Client
        │
        ├─ CRUD / compile ──────────► Meta :8080 ──► Postgres + S3
        │                                  ▲
        └─ POST .../invoke ────────► Worker SDK ───┘  GET /runtime
                                              │
                                              └─ qjs + host bindings
```

## 2. 数据模型

### 2.1 Postgres：`actions`

| 列 | 类型 | 说明 |
|----|------|------|
| `id` | BIGSERIAL | 主键；更新/删除/重编译用 id |
| `name` | VARCHAR(128) | 标识符；invoke / get 路径用 name |
| `label` | VARCHAR(256) | 展示名；空则等于 name |
| `group` | VARCHAR(128) | 分组；默认 `''` |
| `description` | TEXT | 描述 |
| `source_type` | VARCHAR(64) | 默认 `TYPESCRIPT` |
| `source_url` | TEXT | OSS 上 TS 对象 URL |
| `js_url` | TEXT | OSS 上 JS 对象 URL |
| `etag` | VARCHAR(64) | `sha256(source_type + source)`，用于缓存校验 |
| `async` | BOOLEAN | 预留；当前 invoke 仍同步等待结果 |
| `timeout` | INTEGER | 默认超时（**秒**），默认 `60` |
| `version` | BIGINT | 每次 Update 自增 |
| `created_at` / `updated_at` | TIMESTAMPTZ | |
| `deleted_at` | TIMESTAMPTZ | 软删除；非空则不可见 |

**唯一约束**：`(group, name)` 在 `deleted_at IS NULL` 时唯一。

### 2.2 OSS 对象键

| 类型 | Key 形态 |
|------|----------|
| TS 源码 | `action-js/{name}/{etag}.ts` |
| 编译 JS | `action-js/{name}/{etag}.js` |

DB 中只存完整 URL（`source_url` / `js_url`），不存源码正文。

### 2.3 HTTP DTO：`ActionDTO`

| 字段 | JSON | 说明 |
|------|------|------|
| id | `id` | |
| name / label / group / description | 同名 | |
| content | `content` | **仅** get / create / update / compile 响应中带出（从 OSS 读 TS）；**list 不含 content** |
| source_type | `source_type` | |
| source_url / js_url | 同名 | |
| async / timeout / version / etag | 同名 | |
| created_at / updated_at | RFC3339 字符串 | |

## 3. 命名与默认模板

**Name 规则**（Meta create 校验）：

```text
^[a-zA-Z][a-zA-Z0-9_-]*$
```

必须以字母开头，仅允许字母、数字、`_`、`-`。

**默认 Handler**（`POST /api/actions` 且 `content` 为空时写入）：

```ts
import type { ActionRequest, ActionResponse } from 'lowcode-faas/runtime'

export default function handler(req: ActionRequest): ActionResponse {
  return {
    status: 200,
    data: {
      ok: true,
      context: req.context ?? null,
      body: req.body ?? null,
      data: req.data ?? null,
      query: req.query ?? null,
    },
  }
}
```

## 4. Handler 约定

### 4.1 导出形式

编译后的 ESM 必须能解析到可调用的 handler，任选其一：

- `export default function handler(...)`
- `export default async function handler(...)`
- `export function handler(...)`
- `export async function handler(...)`

### 4.2 入参 / 出参（HTTP 模式）

Invoke 按 HTTP Request / Response 建模。Handler 收到 `ActionRequest`，返回 `ActionResponse`：

```ts
import type { ActionRequest, ActionResponse } from 'lowcode-faas/runtime'

export default function handler(req: ActionRequest): ActionResponse {
  return { status: 200, data: { ok: true, query: req.query, body: req.body } }
}
```

| Request 字段 | 说明 |
|--------------|------|
| `context` | 调用方 / 鉴权 / 租户上下文（兼容旧字段 `ctx`） |
| `body` | 请求体 |
| `data` | 路径 / 路由绑定数据 |
| `query` | 查询参数 |
| `method` | HTTP 方法（缺省 `POST`） |
| `headers` | 请求头 |
| `path` | 请求路径（可选） |

| Response 字段 | 说明 |
|---------------|------|
| `status` | HTTP 状态码；同时作为 invoke 的 HTTP status |
| `data` | 响应体数据（旧字段 `body` 会映射为 `data`） |

要点：

- 推荐直接传 flat 字段；也兼容 legacy `{ input: ActionRequest }`
- 裸返回值会被包成 `{ status: 200, data: value }`
- 响应 **没有** `logs` 字段；脚本日志只进 Worker slog
- Promise 会被 await；运行失败时 HTTP **500**，body 为 `{ status: 500, data: { error } }`
- 成功时 meta 在响应头：`X-Faas-Run-Id` / `X-Faas-Duration-Ms` / `X-Faas-Action` / `X-Faas-Etag`

### 4.3 Host 绑定（可选）

Worker SDK 在全局 `host` 上注入 Go 能力（**不要** `import host`；见 `js/runtime.d.ts`、`examples/host-bindings/`、`examples/worker-embed/`）：

| API | 作用 |
|-----|------|
| `host.log` / `host.echo` / `host.upper` / `host.nowMs` | 基础工具（`WithDefaultHost`） |
| `host.mem` + `memSet` / `memGet` / `memLen` | 进程内 KV（ProxyValue，勿序列化） |
| `host.goCtx` / `host.goCtxDeadlineMs` | 透传 Go `context.Context` |
| 自定义 `host.*` | 客户用 `worker.WithHost` 注册；编辑器类型由客户自维护 `.d.ts` |

## 5. 生命周期

```
Create ──► compile(TS→JS) ──► upload .ts+.js ──► INSERT row (version=1)
                │
Update/Compile ─┤
                ▼
         新 etag → 新 OSS 对象 → UPDATE urls/etag，version++
                │
Delete ─────────┴──► soft delete (deleted_at=now)
                │
Invoke ─────────┴──► Worker 拉 runtime → LRU(etag) → qjs 执行
```

要点：

1. **Create / Update / Compile** 都会走 esbuild 编译；失败则 **不写库 / 不更新**（Create 在编译失败时直接 400）。
2. **etag** 变了才意味着源码变了；Worker 用 `If-None-Match` 做 304，避免重复拉 JS。
3. **list** 只读元数据，不拉 OSS。
4. **get** 会按 `source_url` 拉 TS，填入 `content`。

## 6. Meta API（`:8080`）

错误体统一：`{ "ok": false, "error": "..." }`。

### 6.1 `GET /api/actions`

查询参数：

| 参数 | 说明 |
|------|------|
| `group` | 精确匹配分组 |
| `q` | 对 name / label / group / description 做大小写不敏感模糊搜索 |

响应：

```json
{ "items": [ /* ActionDTO，无 content */ ] }
```

### 6.2 `GET /api/actions/{name}`

可选 `?group=`。无 group 时按 name 取第一条（`ORDER BY id LIMIT 1`）。

响应：完整 `ActionDTO`，含 `content`（OSS 可读时）。

### 6.3 `POST /api/actions` → **201**

请求示例：

```json
{
  "name": "hello-world",
  "label": "Hello",
  "group": "",
  "description": "",
  "content": "",
  "timeout": 60
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `name` | 是 | 见命名规则 |
| `content` | 否 | 空则用默认模板 |
| `label` | 否 | 默认 = name |
| `group` | 否 | 默认 `''` |
| `source_type` | 否 | 归一为 TYPESCRIPT |
| `async` / `timeout` | 否 | timeout 默认 60 秒 |

冲突 `(group, name)` → **409**。

### 6.4 `PUT /api/actions/{id}`

部分更新。若带 `content`（或未带但能从 OSS 读到源码），会 **重新编译并上传**，刷新 `etag` / urls，`version++`。

### 6.5 `POST /api/actions/{id}/compile`

从现有 `source_url` 拉 TS，重新编译上传（不改业务字段，仍 bump version）。

### 6.6 `DELETE /api/actions/{id}`

软删除。响应：`{ "status": "deleted" }`。

### 6.7 `GET /api/actions/{name}/runtime`（Worker 内部）

可选 `?group=`。请求头 `If-None-Match: <etag>` 且匹配时 → **304**（不返回 JS body）。

成功 **200**：

```json
{
  "name": "hello-world",
  "group": "",
  "etag": "...",
  "timeout": 60,
  "js_url": "http://.../action-js/hello-world/....js",
  "js": "export default function handler(){...}"
}
```

无 `js_url` → **400**（需先 create/compile 成功）。

## 7. Worker API（`:9090`）

### 7.1 `POST /api/actions/{name}/invoke`

可选 `?group=`。

请求（推荐 flat HTTP 字段）：

```json
{
  "method": "POST",
  "context": { "userId": "u1" },
  "query": { "page": "1" },
  "data": {},
  "headers": { "content-type": "application/json" },
  "path": "/api/hello",
  "body": { "hello": "world" },
  "timeout_ms": 5000
}
```

| 字段 | 说明 |
|------|------|
| `context` / `body` / `data` / `query` / `method` / `headers` / `path` | 组装为 handler 的 `ActionRequest` |
| `input` | legacy：整份 ActionRequest（或 partial） |
| `timeout_ms` | 可选；覆盖 action 的 `timeout`（秒→本次用毫秒） |

响应 body（仅此 shape）：

```json
{
  "status": 200,
  "data": { "ok": true }
}
```

HTTP status = `status`。运行元数据在响应头：

| Header | 说明 |
|--------|------|
| `X-Faas-Run-Id` | 本次 invoke id |
| `X-Faas-Duration-Ms` | 执行耗时 |
| `X-Faas-Action` | action name |
| `X-Faas-Etag` | 源码 etag |

解析流程：

1. 查本地 LRU（key ≈ `group+name`）
2. 命中则带 `If-None-Match` 问 Meta；304 则复用缓存 JS
3. Miss / 变更则拉 `/runtime`，写入 LRU
4. qjs 执行，返回 JSON 结果

Worker 日志为 JSON（slog）：含 `run_id`、`cache`、`status`、`duration_ms` 等。LRU 大小由 `LOWCODE_FAAS_JS_CACHE_SIZE` 控制（默认 128）。

## 8. 端到端示例

```bash
# 1. 创建（使用默认模板）
curl -sS -X POST http://127.0.0.1:8080/api/actions \
  -H 'Content-Type: application/json' \
  -d '{"name":"hello-world"}'

# 2. 列表 / 搜索
curl -sS 'http://127.0.0.1:8080/api/actions?q=hello'

# 3. 更新源码并重新部署
curl -sS -X PUT http://127.0.0.1:8080/api/actions/1 \
  -H 'Content-Type: application/json' \
  -d '{"content":"export default function handler({ body }) { return { echo: body } }\n"}'

# 4. Invoke
curl -sS -D - -X POST http://127.0.0.1:9090/api/actions/hello-world/invoke \
  -H 'Content-Type: application/json' \
  -d '{"context":{},"body":{"x":1},"query":{}}'
```

## 9. Playground 对应关系

独立仓库 `examples/lowcode-faas-playground`（见 monoposer/examples）：

| UI | API |
|----|-----|
| 列表模式（表格 + 搜索） | `GET /api/actions?q=` |
| Deploy a new action | `POST /api/actions`（可无 content） |
| Code → Deploy updates | `PUT /api/actions/{id}` |
| Compile | `POST /api/actions/{id}/compile` |
| Test → Send request | `POST /api/actions/{name}/invoke` |
| Delete | `DELETE /api/actions/{id}` |

Playground 经 Vite 代理：CRUD → Meta `:8080`，invoke → Worker `:9090`。

## 10. 相关代码索引

| 主题 | 路径 |
|------|------|
| 模型 | `internal/model/action.go` |
| Meta handlers / DTO | `internal/api/handlers.go`, `internal/api/dto.go` |
| Worker SDK | `worker/` |
| Worker invoke | `internal/api/worker.go`, `internal/api/httpaction.go` |
| Postgres | `internal/store/postgres.go` |
| 迁移 | `migrations/000001_init.up.sql` |
| TS 编译 / 默认模板 / OSS key | `internal/tscompile/` |
| qjs runner + host | `internal/runner/` |
| 类型声明 | `js/runtime.d.ts` |
| Host / embed 示例 | `examples/host-bindings/`, `examples/worker-embed/` |

## 11. 设计约束（当前版本）

- 源码与 JS **必须**进 OSS；Meta 未配置 uploader/compiler 时 create/update 会失败。
- 多 group 下同名 action：无 `?group=` 时 get/invoke/runtime 取 id 最小的一条；生产调用建议始终带 `group`。
- `async` 字段已落库，对外 invoke 仍为同步请求/响应。
- 软删除后 `(group, name)` 可再创建同名；OSS 旧对象不会自动清理。
