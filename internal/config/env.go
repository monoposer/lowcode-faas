package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// LoadFromEnv 用环境变量覆盖默认配置
func LoadFromEnv() {
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_STORAGE")); v != "" {
		StorageBackend = v
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_POSTGRES_DSN")); v != "" {
		PostgresDSN = v
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_S3_ENDPOINT")); v != "" {
		S3Endpoint = v
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_S3_REGION")); v != "" {
		S3Region = v
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_S3_BUCKET")); v != "" {
		S3Bucket = v
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_S3_ACCESS_KEY")); v != "" {
		S3AccessKey = v
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_S3_SECRET_KEY")); v != "" {
		S3SecretKey = v
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOWCODE_FAAS_S3_USE_PATH_STYLE"))) {
	case "1", "true", "yes":
		S3UsePathStyle = true
	case "0", "false", "no":
		S3UsePathStyle = false
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOWCODE_FAAS_FUNCTION_CACHE"))) {
	case "0", "false", "no", "off":
		FunctionCacheEnabled = false
	case "1", "true", "yes", "on":
		FunctionCacheEnabled = true
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_WORKER_URL")); v != "" {
		WorkerBaseURL = strings.TrimRight(v, "/")
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_WORKER_LISTEN")); v != "" {
		WorkerListen = v
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_INVOKE_DEFAULT_TIMEOUT_MS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			DefaultInvokeTimeout = time.Duration(n) * time.Millisecond
		}
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_NODE_DOCKER_IMAGE")); v != "" {
		NodeDockerImage = v
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOWCODE_FAAS_PREWARM_NODE_IMAGE"))) {
	case "0", "false", "no", "off":
		PrewarmNodeImage = false
	case "1", "true", "yes", "on":
		PrewarmNodeImage = true
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOWCODE_FAAS_FUNCTION_LOGS"))) {
	case "1", "true", "yes", "on":
		FunctionLogsEnabled = true
	case "0", "false", "no", "off":
		FunctionLogsEnabled = false
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_FUNCTION_LOGS_SINK")); v != "" {
		FunctionLogsSink = v
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_FUNCTION_LOGS_BUFFER")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			FunctionLogsBufferMax = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_LOG_ES_URL")); v != "" {
		FunctionLogsESURL = v
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_LOG_ES_INDEX")); v != "" {
		FunctionLogsESIndex = v
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_LOG_ES_USER")); v != "" {
		FunctionLogsESUser = v
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_LOG_ES_PASSWORD")); v != "" {
		FunctionLogsESPassword = v
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_LOG_HTTP_URL")); v != "" {
		FunctionLogsHTTPURL = v
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_LOG_HTTP_HEADER")); v != "" {
		FunctionLogsHTTPHeader = v
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_LOG_LOKI_URL")); v != "" {
		FunctionLogsLokiURL = strings.TrimRight(v, "/")
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_LOG_LOKI_TENANT")); v != "" {
		FunctionLogsLokiTenant = v
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_LOG_LOKI_TIMEOUT_MS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			FunctionLogsLokiTimeout = time.Duration(n) * time.Millisecond
		}
	}
	if v := strings.TrimSpace(os.Getenv("LOWCODE_FAAS_LOGS_GRAFANA_EXPLORE_URL_TEMPLATE")); v != "" {
		LogsGrafanaExploreURLTemplate = v
	}
}
