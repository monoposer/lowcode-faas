package functionlogs

import "time"

// Entry 单条用户 console 输出（来自容器内 NDJSON）。
type Entry struct {
	RunID              string    `json:"run_id"`
	ExecutionID        string    `json:"execution_id,omitempty"`
	FunctionName       string    `json:"function_name"`
	CollectionPath     string    `json:"collection_path,omitempty"`
	DeploymentVersion  int       `json:"deployment_version,omitempty"`
	DeploymentID       string    `json:"deployment_id,omitempty"`
	TS                 time.Time `json:"ts"`
	Level              string    `json:"level"`
	Message            string    `json:"message"`
	// LogStream 写入 Docker stdout / Promtail 时设为固定值，便于 LogQL 过滤；内存检索等场景可留空
	LogStream string `json:"log_stream,omitempty"`
}
