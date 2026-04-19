package config

import "time"

// 容器与运行时常量

const ContainerHTTPPort = 8080 // 历史保留；当前 JS 批处理模式不再监听端口
const ServerReadyWait = 20

var (
	// NodeDockerImage 执行用户 handler 的镜像。Docker 会在本机缓存镜像层；首次 invoke 或 prune 后
	// 才会出现 “Unable to find image locally” 并拉取。浮动 tag 在上游更新后可能触发增量拉取，生产可改用 digest 固定。
	NodeDockerImage = "node:22-alpine"

	DockerFunctionMemory = "256m"
	DockerFunctionCPUs   = "1"
	DockerNetworkMode    = ""

	MaxConcurrentFunctionRuns = 32

	SourceFetchTimeoutSeconds       = 60
	SourceFetchMaxBytes       int64 = 1 << 20

	AllowSourceFetchToPrivateNetworks = false

	StorageBackend = "files"
	PostgresDSN    = ""
	S3Endpoint     = ""
	S3Region       = "us-east-1"
	S3Bucket       = ""
	S3AccessKey    = ""
	S3SecretKey    = ""
	S3UsePathStyle = true

	FunctionCacheEnabled = true

	// WorkerBaseURL 非空时，dispatcher 将 invoke 转发给该地址（如 http://127.0.0.1:9090）；空则本进程内直接执行
	WorkerBaseURL = ""

	// WorkerListen worker 进程监听地址，默认 :9090
	WorkerListen = ":9090"

	// DefaultInvokeTimeout 请求体未带 timeout_ms 时的上限；须覆盖「首次 docker pull 镜像」耗时，过短会导致 pull 被 cancel 报 signal: killed
	DefaultInvokeTimeout = 120 * time.Second

	// PrewarmNodeImage 启动后后台 docker pull NodeDockerImage（仅在本进程会执行 docker 时启用）
	PrewarmNodeImage = true

	// FunctionLogs 用户函数 console 采集（关闭时行为与原先一致：仅容器 stdout）
	FunctionLogsEnabled = false
	// FunctionLogsSink: stdout | docker_json | promtail | memory | elasticsearch | es | http | loki（push，非推荐）
	FunctionLogsSink = "stdout"
	// FunctionLogsBufferMax 内存可检索条数上限
	FunctionLogsBufferMax = 10000

	FunctionLogsESURL      = ""
	FunctionLogsESIndex    = "lowcode-faas-logs"
	FunctionLogsESUser     = ""
	FunctionLogsESPassword = ""

	FunctionLogsHTTPURL    = ""
	FunctionLogsHTTPHeader = "" // 可选，如 "Authorization: Bearer xxx"

	// FunctionLogsLokiURL 例如 http://127.0.0.1:3100（不含路径）；sink=loki 时推送至 /loki/api/v1/push
	FunctionLogsLokiURL     = ""
	FunctionLogsLokiTenant  = "" // 可选 X-Scope-OrgID
	FunctionLogsLokiTimeout = 8 * time.Second

	// LogsGrafanaExploreURLTemplate 可选；GET /function-logs/query 替换 {{logql}}、{{run_id}}（已 URL 编码）生成跳转链接
	LogsGrafanaExploreURLTemplate = ""
)
