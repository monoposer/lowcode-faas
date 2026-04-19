package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const LangJavaScript = "javascript"

// Function 用户函数（当前版本仅支持 JavaScript / Node ESM）
type Function struct {
	ID               string            `json:"id"`
	CollectionPath   string            `json:"collection_path,omitempty"`
	Name             string            `json:"name"`
	Language         string            `json:"language"`
	SourceCode       string            `json:"source_code"`
	Env              map[string]string `json:"env,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	Version      int    `json:"version"`
	DeploymentID string `json:"deployment_id,omitempty"`
}

// FormatDeploymentID 稳定可读部署标识（日志 / RunResult）
func FormatDeploymentID(collectionPath, name string, version int) string {
	if strings.TrimSpace(collectionPath) != "" {
		return fmt.Sprintf("%s/%s@v%d", strings.Trim(collectionPath, "/"), name, version)
	}
	return fmt.Sprintf("%s@v%d", name, version)
}

// FunctionListItem 列表项（不含源码）
type FunctionListItem struct {
	ID             string    `json:"id"`
	CollectionPath string    `json:"collection_path,omitempty"`
	Name           string    `json:"name"`
	Language       string    `json:"language"`
	Version        int       `json:"version"`
	CreatedAt      time.Time `json:"created_at"`
}

// FunctionVersionItem 单条部署版本（db_s3）
type FunctionVersionItem struct {
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
}

// CollectionListItem GET /collections
type CollectionListItem struct {
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// CreateCollectionRequest POST /collections — mkdir -p 语义
type CreateCollectionRequest struct {
	Path string            `json:"path"`
	Env  map[string]string `json:"env,omitempty"`
}

// RunResult 一次执行结果（与历史 API 字段保持兼容）
type RunResult struct {
	RunID              string            `json:"run_id"`
	ExecutionID        string            `json:"execution_id,omitempty"`
	Status             string            `json:"status"`
	HTTPStatusCode     int               `json:"http_status_code"`
	HTTPHeaders        map[string]string `json:"http_headers,omitempty"`
	Output             json.RawMessage   `json:"output"`
	ErrorMessage       string            `json:"error_message"`
	DurationMs         int64             `json:"duration_ms"`
	FunctionID         string            `json:"function_id"`
	CollectionPath     string            `json:"collection_path,omitempty"`
	DeploymentVersion  int               `json:"deployment_version,omitempty"`
	DeploymentID       string            `json:"deployment_id,omitempty"`
}

// NewRunID 生成 run id
func NewRunID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// CreateFunctionRequest POST /functions
type CreateFunctionRequest struct {
	Name           string            `json:"name"`
	Language       string            `json:"language"`
	SourceCode     string            `json:"source_code"`
	SourceURL      string            `json:"source_url,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	CollectionPath string            `json:"collection_path,omitempty"`
}

// InvokeFunctionRequest POST .../invoke
type InvokeFunctionRequest struct {
	Input     json.RawMessage   `json:"input"`
	TimeoutMs int               `json:"timeout_ms"`
	Env       map[string]string `json:"env,omitempty"`
	Version   *int              `json:"version,omitempty"`
}

// PullFunctionRequest POST .../pull
type PullFunctionRequest struct {
	SourceURL string `json:"source_url"`
}

// UpdateFunctionRequest PUT .../{name}
type UpdateFunctionRequest struct {
	SourceCode string             `json:"source_code"`
	Env        *map[string]string `json:"env,omitempty"`
}

// WorkerRunRequest dispatcher → worker 内部协议
type WorkerRunRequest struct {
	CollectionPath string            `json:"collection_path,omitempty"`
	FunctionName   string            `json:"function_name"`
	Input          json.RawMessage   `json:"input"`
	TimeoutMs      int               `json:"timeout_ms"`
	Env            map[string]string `json:"env,omitempty"`
	Version        *int              `json:"version,omitempty"`
}
