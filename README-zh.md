# lowcode-faas

TypeScript Action FaaS，**meta（控制面）** 与可嵌入的 **worker SDK** 分离：

- **Meta**（`cmd/meta`，`:8080`）：Action CRUD；保存时 **TS → JS（esbuild）**；元数据进 Postgres；**source 与 JS 一律上传到 S3 OSS**。对 worker 提供 `GET /api/actions/{name}/runtime`。可用 Docker（见 `Dockerfile`）或 `make run` 启动。
- **Worker**（[`worker`](./worker) 包）：**对外 invoke**，嵌入*你的* Go 服务；通过 meta client 拉元数据 + 已编译 JS，再用 [fastschema/qjs](https://github.com/fastschema/qjs) 执行。用 `LOWCODE_FAAS_META_URL` 等环境变量配置；用 `worker.WithHost` 注入自定义 Go host。

```
Client → Meta :8080          CRUD / 编译 → Postgres + S3 OSS
Client → 你的服务             worker SDK :9090（示例）POST .../invoke
                               └─ GET Meta /api/actions/{name}/runtime
                               └─ qjs 执行 → JSON
```

Handler 约定：

```ts
import type { ActionRequest, ActionResponse } from 'lowcode-faas/runtime'

export default function handler(req: ActionRequest): ActionResponse {
  return { status: 200, data: { ok: true } }
}
```

`host` 为 Worker 全局注入；基础类型见 `js/runtime.d.ts`。自定义 host 类型由客户前端自维护（demo：`examples/worker-embed/host.d.ts`）。

**Action 技术文档**：[docs/action.md](./docs/action.md)（数据模型、生命周期、API、示例）。

## 快速开始

```bash
cp .env.example .env
make docker-up

make run          # meta :8080（或 docker compose --profile meta up -d --build）
make run-worker   # examples/worker-embed :9090（需 LOWCODE_FAAS_META_URL）
```

嵌入自己的服务：

```go
import "lowcode-faas/worker"

w, err := worker.New(worker.ConfigFromEnv(),
  worker.WithDefaultHost(),
  worker.WithHost(myBinder),
)
_ = w.Run(ctx)
```

完整示例：[examples/worker-embed](./examples/worker-embed/)。

Playground：`examples/lowcode-faas-playground`。

## Meta API（`:8080`）

| Method | Path | 说明 |
|--------|------|------|
| GET/POST | `/api/actions` | 列表（`?group=` / `?q=`）/ 创建（空 content → 默认模板，201） |
| GET | `/api/actions/{name}` | 详情（含 TS content） |
| GET | `/api/actions/{name}/runtime` | 给 worker：元数据 + 编译后 `js` |
| PUT/DELETE/compile | … | 更新 / 软删 / 重编译 |

## Worker API（SDK 监听地址，默认 `:9090`）

| Method | Path | 说明 |
|--------|------|------|
| POST | `/api/actions/{name}/invoke` | `{ context?, body?, data?, query?, … }` → HTTP 风格 Request/Response |

## 环境变量

见 `.env.example`。Worker SDK 必填 `LOWCODE_FAAS_META_URL`。
