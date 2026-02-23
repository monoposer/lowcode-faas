package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const dataDir = "data"

var ErrNotFound = errors.New("not found")

// Function 表示用户提交的函数定义
type Function struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Language   string    `json:"language"`    // python | deno
	SourceCode string    `json:"source_code"` // 用户代码
	CreatedAt  time.Time `json:"created_at"`
}

// RunResult 表示一次执行结果（函数为 HTTP 服务，返回其 HTTP 响应）
type RunResult struct {
	RunID          string            `json:"run_id"`
	Status         string            `json:"status"`           // success | failed | timeout
	HTTPStatusCode int              `json:"http_status_code"`  // 函数返回的 HTTP 状态码
	HTTPHeaders    map[string]string `json:"http_headers,omitempty"` // 函数返回的响应头（可选）
	Output         json.RawMessage   `json:"output"`           // 响应体（原始）
	ErrorMessage   string           `json:"error_message"`    // 平台错误或超时信息
	DurationMs      int64            `json:"duration_ms"`
	FunctionID      string           `json:"function_id"`
}

// 创建简单 ID（时间戳）
func newID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// 创建函数的请求体
type createFunctionRequest struct {
	Name       string `json:"name"`
	Language   string `json:"language"`
	SourceCode string `json:"source_code"`
}

// 调用函数的请求体
type invokeFunctionRequest struct {
	Input     json.RawMessage `json:"input"`
	TimeoutMs int             `json:"timeout_ms"`
}
