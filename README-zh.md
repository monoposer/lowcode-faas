# lowcode-faas

轻量 FaaS（**第一版仅 JavaScript**）：用户代码为 **Node ESM**，导出固定签名的 **`handler`**；调度进程 **dispatcher** 提供 HTTP API，可选将执行转发给独立进程 **worker**。源码可落在本地 `data/`，或 **PostgreSQL + S3**（`db_s3`）。

**English:** [README.md](README.md)

## 进程角色

| 命令 | 说明 |
|------|------|
| `go run ./cmd/dispatcher` | 对外 `POST /functions`、`POST /functions/{name}/invoke`；若设置 `LOWCODE_FAAS_WORKER_URL` 则把执行 HTTP 转发给 worker |
| `go run ./cmd/worker` | 对内 `POST /v1/run`，读存储并起 **Docker** 跑用户 `handler` |

单机开发可只跑 **dispatcher**（不设 `WORKER_URL`），此时与 worker 同逻辑在本进程执行。

## 用户代码契约（固定结构）

- 文件为 **ESM**，默认文件名在磁盘上为 `{name}.mjs`。
- **必须**导出：

```javascript
/** @param {import('../../js/runtime.d.ts').HandlerInput} input */
export async function handler(input) {
  return { ok: true, data: { message: "..." } };
}
```

- **入参 `input`**：与 `invoke` 请求 JSON 里的 `input` 字段一致（任意 JSON 对象）。
- **返回值**（写入 `RunResult.output`）必须是带布尔字段 **`ok`** 的对象：
  - 成功：`{ "ok": true, "data": <任意> }`
  - 业务失败：`{ "ok": false, "error": "说明", "code": "可选" }`

TypeScript 可参考仓库内 [`js/runtime.d.ts`](js/runtime.d.ts)。

执行方式：平台在容器内写入 `input.json`，运行内嵌 `bootstrap.mjs` 调用 `handler`，再读取 `output.json`（**不再**由用户代码起 HTTP 端口）。

## 构建

```bash
go build -o dispatcher ./cmd/dispatcher
go build -o worker ./cmd/worker
```

## 配置与环境变量

核心变量在 [`internal/config/config.go`](internal/config/config.go)，可被环境变量覆盖（见 [`internal/config/env.go`](internal/config/env.go)）。

| 变量 | 含义 |
|------|------|
| `LOWCODE_FAAS_LISTEN` | dispatcher 监听，默认 `:8080` |
| `LOWCODE_FAAS_WORKER_URL` | 非空则 invoke 转发到该 base（如 `http://127.0.0.1:9090`） |
| `LOWCODE_FAAS_WORKER_LISTEN` | worker 监听，默认 `:9090` |
| `LOWCODE_FAAS_STORAGE` | `files`（默认）或 `db_s3` |
| `LOWCODE_FAAS_POSTGRES_DSN` | `db_s3` 时 PostgreSQL 连接串 |
| `LOWCODE_FAAS_S3_*` | `db_s3` 时 S3 / MinIO 配置 |
| `LOWCODE_FAAS_FUNCTION_CACHE` | `true` / `false` |
| `LOWCODE_FAAS_INVOKE_DEFAULT_TIMEOUT_MS` | 请求未带 `timeout_ms` 时的默认上限（毫秒），默认 `120000`；过短会在首次 `docker pull` 时触发 `signal: killed` |
| `LOWCODE_FAAS_NODE_DOCKER_IMAGE` | 执行用户 `handler` 的镜像，默认 `node:22-alpine`；内网可改为私有仓库地址 |
| `LOWCODE_FAAS_PREWARM_NODE_IMAGE` | 默认 `true`：进程启动后后台 `docker pull` 上述镜像；设为 `false` 可关闭 |
| `LOWCODE_FAAS_FUNCTION_LOGS` / `LOWCODE_FAAS_FUNCTION_LOGS_SINK` | 用户 `console.*` 采集。**推荐** `docker_json` 或 `promtail`：每行一条 JSON 打到进程 stdout → Docker json-file → **Promtail → Loki**（见 `docker-compose.dev.yml`）。`stdout` 仍为带前缀的兼容格式。直连 Loki push 用 `loki` + `LOWCODE_FAAS_LOG_LOKI_*`（一般不用于与 Promtail 重复） |
| `LOWCODE_FAAS_LOGS_GRAFANA_EXPLORE_URL_TEMPLATE` | 可选；`GET /function-logs/query` 在返回 JSON 里生成 `grafana_explore_url`，占位符 `{{logql}}`、`{{run_id}}`、`{{deployment_id}}`（已 URL 编码） |

执行用户代码时采用 **`docker create` + `docker cp` 工作区进容器 + `docker start`**，再 **`docker cp` 回 `output.json`**，避免 worker 在容器内挂载「仅存在于兄弟容器 /tmp」的路径导致子容器里缺少 `bootstrap.mjs`（`MODULE_NOT_FOUND`）。

**中国区 / Docker 镜像加速（registry mirror，不是 HTTP 代理）**：`invoke` 时 `docker pull` 由**宿主机** Docker 守护进程执行（挂载 `docker.sock`）。拉 Hub 慢应配置 **镜像加速器**，例如 Docker Desktop → **Settings → Docker Engine**，在 JSON 里增加 `registry-mirrors`（示例：`"registry-mirrors": ["https://docker.m.daocloud.io"]`，具体地址以你使用的镜像站文档为准）。Linux 则编辑 **`/etc/docker/daemon.json`** 后重启 Docker。  
**不配置 daemon 时的应用侧做法**：把 `LOWCODE_FAAS_NODE_DOCKER_IMAGE` 设为镜像站上的**完整镜像名**（如 `docker.m.daocloud.io/library/node:22-alpine`），等价于直接从国内仓库拉，而仍写 `node:22-alpine` 则走默认 registry，与 `HTTP_PROXY` 无关。

## API

### 集合（嵌套路径）

- `POST /collections` body：`{ "path": "org/team", "env": { ... } }`（`env` 可选）— 登记路径；非根集合下创建函数前须先创建。
- `GET /collections?prefix=org` — 列出已登记路径（前缀过滤，可选）。
- `PUT /collections/{path}/env` 或 `PUT /collections/env`（根集合）— body `{ "env": { ... } }` 写集合级环境变量。
- 亦可通过 URL 挂载：`/collections/org/team/functions`、`/collections/org/team/functions/{name}/invoke` 等（路径中 `org/team` 为 `collection_path`）。

### 创建函数 `POST /functions`

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

`collection_path` 可省略（根集合）；也可在请求上使用查询参数 `?collection_path=org%2Fteam`。

`language` 仅支持 **`javascript`**（可省略，默认即为 `javascript`）。

### 列出函数 `GET /functions`

查询参数 `collection_path` 指定集合；省略则为根集合。

### 列出部署版本 `GET /functions/{name}/versions`

仅 `db_s3` 返回完整历史；`files` 仅返回当前版本一条。集合路径同样用 `?collection_path=` 或嵌套 URL。

### 调用 `POST /functions/{name}/invoke`

```json
{ "input": { "name": "lowcode" }, "timeout_ms": 15000, "env": {}, "version": 3 }
```

可选 **`version`** 固定调用某部署版本（仅 `db_s3` 任意历史版本；`files` 仅当与当前版本一致时有效）。也可用查询参数 `?version=3`。集合：`?collection_path=org%2Fteam`。

响应仍为 `RunResult`（含 `execution_id`、`deployment_version`、`deployment_id`、`collection_path` 等）；业务结果在 **`output`**（即 `handler` 的返回值 JSON）。

### 环境变量合并顺序

一次 invoke 注入容器的进程环境为：**祖先集合 env（根→…→当前路径，子覆盖父）∪ 函数 env ∪ 请求体 `env`**，后者优先级最高。

### 执行日志与 Loki / Promtail（推荐）

1. 设置 `LOWCODE_FAAS_FUNCTION_LOGS=true`、`LOWCODE_FAAS_FUNCTION_LOGS_SINK=docker_json`（与 `promtail` 等价）。
2. 日志为**单行 JSON**（含 `run_id`、`execution_id`、`log_stream`、`function_name`、`collection_path`、`deployment_id` 等），进入 **worker/dispatcher 容器 stdout**，由 **Promtail**（`deploy/promtail-dev.yml`）经 Docker API 采集并写入 **Loki**。本地可 `docker compose -f docker-compose.dev.yml up`（含 `loki:3100`）。
3. **查询辅助 URL**：`GET /function-logs/query?run_id=...&function=...&collection_path=...&deployment_id=...` 返回 JSON，内含各后端**示例查询串**（`loki_logql_json`、`loki_logql_substr`、`elasticsearch_query_string`、`aws_cloudwatch_logs_insights`、`google_cloud_logging`）。Elasticsearch、Loki、CloudWatch、GCP 的查询语言**无法完全统一**；做法是统一 **JSON 字段名**，在各平台用等价条件过滤同一 `run_id` / `deployment_id`。
4. 若配置 `LOWCODE_FAAS_LOGS_GRAFANA_EXPLORE_URL_TEMPLATE`，响应中可带 `grafana_explore_url`（模板里用 `{{logql}}` 等占位符由服务替换）。

### 拉取更新 `POST /functions/{name}/pull`

```json
{ "source_url": "https://example.com/handler.mjs" }
```

## 数据目录（files 模式）

- `data/collections/__root__/functions/{name}.mjs` — 根集合源码（当前默认路径）
- `data/functions/{name}.mjs` — **历史**根集合源码；若仍存在，列举/加载根函数时会一并识别
- `data/collections/__root__/functions/{name}.env.json` — 与上述根 `.mjs` 同目录的函数 env
- `data/functions/{name}.env.json` — 仅配合历史根 `data/functions/{name}.mjs` 的 env
- `data/collections/{segment}/.../{segment}/functions/{name}.mjs` — 非根集合源码
- `data/collections/__root__/collection.env.json` — 根集合级 env（可选）
- `data/collections/{path}/collection.env.json` — 该路径集合级 env

## systemd

- [`lowcode-faas.service`](lowcode-faas.service)：运行 **dispatcher**（将 `ExecStart` 改为 `dispatcher` 二进制路径）。
- 若使用 worker：另建单元运行 `worker`，并为 dispatcher 配置 `Environment=LOWCODE_FAAS_WORKER_URL=http://127.0.0.1:9090`。

## 目录结构（整理后）

```
cmd/dispatcher/main.go   # 调度 HTTP
cmd/worker/main.go       # 执行 HTTP
internal/config/         # 配置与 env 覆盖
internal/model/          # 类型与契约说明
internal/store/          # files + PostgreSQL/S3
internal/runner/         # Docker + bootstrap.mjs
internal/httpserver/     # 调度路由
internal/workerserver/   # worker 路由
internal/workerclient/   # 调度 → worker HTTP
internal/envutil/ internal/limits/ internal/sourcefetch/
js/runtime.d.ts          # TS 类型参考
```
