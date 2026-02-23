# lowcode-faas

轻量级 FaaS：通过 HTTP API 提交代码（Python / Deno），在 Docker 容器内以 **HTTP 服务** 形式运行，平台将请求转发给该服务并返回其 **HTTP 响应**。数据存放在本地 `data/` 目录。

## 依赖

- **Go 1.22+**
- **Docker**（用于运行函数容器）

## 函数约定

- 每个函数是一段 **HTTP 服务** 代码，在容器内监听固定端口（默认 8080，由环境变量 `PORT` 指定）。
- 平台会：启动容器 → 等待服务就绪 → 将调用时的 `input` 以 **POST 请求体** 发到容器内服务的 `/` → 把该请求的 **HTTP 状态码、响应头、响应体** 作为结果返回。
- 函数内的 **print / log** 仅输出到容器控制台（平台不捕获），不影响返回结果；返回内容以该 HTTP 响应的 body 为准。

## 配置常量

容器相关常量集中在 `config.go` 中，便于修改：

- `ContainerHTTPPort`：容器内 HTTP 服务端口（默认 8080）
- `ServerReadyWait`：等待服务就绪的最长时间（秒）
- `PythonDockerImage` / `DenoDockerImage`：Python / Deno 使用的 Docker 镜像

## 快速开始

```bash
go build -o lowcode-faas . && ./lowcode-faas
# 或
go run .
```

服务默认监听 `http://localhost:8080`。

## 使用 systemd 保持常驻

1. 构建并安装到系统路径（例如）：

   ```bash
   go build -o lowcode-faas .
   sudo cp lowcode-faas /usr/local/bin/
   sudo mkdir -p /opt/lowcode-faas
   sudo cp -r data /opt/lowcode-faas/
   ```

2. 安装并启用服务：

   ```bash
   sudo cp lowcode-faas.service /etc/systemd/system/
   # 若路径不同，编辑 /etc/systemd/system/lowcode-faas.service 中的 ExecStart 与 WorkingDirectory
   sudo systemctl daemon-reload
   sudo systemctl enable lowcode-faas
   sudo systemctl start lowcode-faas
   sudo systemctl status lowcode-faas
   ```

3. 常用命令：`journalctl -u lowcode-faas -f` 查看日志；`systemctl restart lowcode-faas` 重启。

## API

### 创建函数

**POST** `/functions`

请求体：

```json
{
  "name": "函数名称",
  "language": "python",
  "source_code": "..."
}
```

- `language`: `python` 或 `deno`
- `source_code`: 完整 HTTP 服务源码（需在 `PORT` 端口启动服务）

响应：返回创建的 `Function`（含 `id`、`name`、`language`、`source_code`、`created_at`）。

### 调用函数

**POST** `/functions/{name}/invoke`

请求体（可选）：

```json
{
  "input": { "key": "value" },
  "timeout_ms": 5000
}
```

- `input`: 任意 JSON，会作为 **POST 请求体** 发给函数容器内的 HTTP 服务
- `timeout_ms`: 超时毫秒数，默认 5000

响应：返回 `RunResult`，包含函数返回的 **HTTP 状态码**（`http_status_code`）、**响应头**（`http_headers`）、**响应体**（`output`），以及 `status`（success/failed/timeout）、`duration_ms`、`function_id` 等。

## 数据目录

- `data/functions/`：仅存放函数源码文件，例如：
  - Python: `data/functions/greet.py`
  - Deno: `data/functions/greet.ts`

函数名即文件名（不含扩展名）。可直接在目录下放置 `xxx.py` 或 `xxx.ts`，然后请求 `POST /functions/xxx/invoke`，无需先调创建接口。

## 示例

项目自带示例（均为 HTTP 服务，监听 `PORT`，POST 读 JSON 返回问候语）：

- **Python**：`data/functions/greet.py`（标准库 `http.server`）
- **Deno**：`data/functions/greet.ts`（`Deno.serve`）

调用示例：

```bash
# 调用 greet（假设已存在 data/functions/greet.py 或 greet.ts）
curl -s -X POST "http://localhost:8080/functions/greet/invoke" \
  -H "Content-Type: application/json" \
  -d '{"input": {"name": "lowcode-faas"}}'
```

返回中的 `output` 为函数 HTTP 响应体，例如 `{"message": "Hello, lowcode-faas"}`；`http_status_code` 为 200。
