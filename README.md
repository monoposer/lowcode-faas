# lowcode-faas

轻量级 FaaS：通过 HTTP API 提交代码（Python / Deno），在 Docker 容器中执行并返回结果。数据持久化在本地 `data/` 目录。

## 依赖

- **Go 1.22+**
- **Docker**（用于运行 Python / Deno 代码）

## 快速开始

```bash
# 构建并运行
go build -o lowcode-faas . && ./lowcode-faas
# 或直接运行
go run .
```

服务默认监听 `http://localhost:8080`。

## API

### 创建函数

**POST** `/functions`

请求体：

```json
{
  "name": "函数名称",
  "language": "python",
  "source_code": "print('hello')"
}
```

- `language`: 仅支持 `python` 或 `deno`
- `source_code`: 函数源码（Python 为 `main.py` 逻辑，Deno 为 `main.ts`）

响应：返回创建的 `Function`（含 `id`、`name`、`language`、`source_code`、`created_at`）。

### 调用函数

**POST** `/functions/{id}/invoke`

请求体（可选）：

```json
{
  "input": { "key": "value" },
  "timeout_ms": 5000
}
```

- `input`: 任意 JSON，会作为 **stdin** 传入用户代码
- `timeout_ms`: 超时毫秒数，默认 5000

响应：返回 `RunResult`（`run_id`、`status`、`output`、`error_message`、`exit_code`、`duration_ms`、`function_id`）。  
若用户程序向 stdout 输出合法 JSON，则 `output` 为该 JSON；否则为字符串包装。

## 数据目录

- `data/functions/` — 仅存放函数源码文件，例如：
  - Python: `data/functions/greet.py`
  - Deno: `data/functions/greet.ts`

不再额外存 JSON 元数据，函数名就是文件名（不含扩展名）。

## 使用示例

先启动服务（见「快速开始」），再按下面步骤体验。

**1. 创建函数**（会写入 `data/functions/greet.py`）

```bash
curl -s -X POST http://localhost:8080/functions \
  -H "Content-Type: application/json" \
  -d '{
    "name": "greet",
    "language": "python",
    "source_code": "import sys, json\nx = json.load(sys.stdin)\nprint(json.dumps({\"message\": \"Hello, \" + x.get(\"name\", \"world\")}))"
  }'
```

**2. 调用函数**（`{id}` 即函数名，如 `greet`）

```bash
curl -s -X POST "http://localhost:8080/functions/greet/invoke" \
  -H "Content-Type: application/json" \
  -d '{"input": {"name": "lowcode-faas"}}'
```

返回的 `output` 中会包含 `{"message": "Hello, lowcode-faas"}`。  
也可直接在 `data/functions/` 下放置 `xxx.py` 或 `xxx.ts`，然后请求 `POST /functions/xxx/invoke` 即可运行，无需先调用创建接口。
