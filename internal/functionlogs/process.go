// Package functionlogs 采集用户 handler 内 console 输出。
// 推荐：sink=docker_json（单行 JSON 到 stdout）+ Docker json-file + Promtail → Loki；亦支持直连 ES / HTTP / 旧版 Loki push / 内存检索。
package functionlogs

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"lowcode-faas/internal/config"
	"lowcode-faas/internal/model"
)

// WorkspaceUserLogFile 容器 /workspace 内用户 console NDJSON 文件名（runner 拷回宿主机用）
const WorkspaceUserLogFile = "faas_user_logs.ndjson"

// ProcessFileAfterRun 读取容器工作区中的用户日志文件并分发到 sink / 内存索引。
func ProcessFileAfterRun(ctx context.Context, runID string, fn *model.Function, workspaceDir string) {
	if !config.FunctionLogsEnabled {
		return
	}
	fnName := ""
	var collPath string
	var depVer int
	var depID string
	if fn != nil {
		fnName = fn.Name
		collPath = fn.CollectionPath
		depVer = fn.Version
		depID = fn.DeploymentID
		if depID == "" {
			depID = model.FormatDeploymentID(collPath, fnName, depVer)
		}
	}
	p := filepath.Join(workspaceDir, WorkspaceUserLogFile)
	b, err := os.ReadFile(p)
	if err != nil || len(bytes.TrimSpace(b)) == 0 {
		return
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	const maxLine = 1 << 20 // 1 MiB per line safety
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, maxLine)

	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}
		e := Entry{
			RunID:             runID,
			ExecutionID:       strings.TrimSpace(toString(raw["execution_id"])),
			FunctionName:      fnName,
			CollectionPath:    strings.TrimSpace(toString(raw["collection_path"])),
			DeploymentID:      strings.TrimSpace(toString(raw["deployment_id"])),
			Level:             strings.TrimSpace(toString(raw["level"])),
			Message:           strings.TrimSpace(toString(raw["message"])),
			DeploymentVersion: parseIntish(raw["deployment_version"]),
		}
		if v, ok := raw["run_id"].(string); ok && strings.TrimSpace(v) != "" {
			e.RunID = strings.TrimSpace(v)
		}
		if v, ok := raw["function_name"].(string); ok && strings.TrimSpace(v) != "" {
			e.FunctionName = strings.TrimSpace(v)
		}
		if tsStr, ok := raw["ts"].(string); ok {
			if t, err := time.Parse(time.RFC3339Nano, tsStr); err == nil {
				e.TS = t.UTC()
			} else if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
				e.TS = t.UTC()
			}
		}
		if e.TS.IsZero() {
			e.TS = time.Now().UTC()
		}
		if e.RunID == "" {
			e.RunID = runID
		}
		if e.ExecutionID == "" {
			e.ExecutionID = e.RunID
		}
		if e.FunctionName == "" {
			e.FunctionName = fnName
		}
		if e.CollectionPath == "" {
			e.CollectionPath = collPath
		}
		if e.DeploymentID == "" {
			e.DeploymentID = depID
		}
		if e.DeploymentVersion == 0 {
			e.DeploymentVersion = depVer
		}
		Append(e)
		dispatchSink(ctx, e)
	}
	if err := sc.Err(); err != nil {
		log.Printf("functionlogs scan: %v", err)
	}
}

func parseIntish(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case json.Number:
		n, _ := strconv.Atoi(string(t))
		return n
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(t))
		return n
	default:
		return 0
	}
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case json.Number:
		return string(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

func dispatchSink(_ context.Context, e Entry) {
	switch strings.ToLower(strings.TrimSpace(config.FunctionLogsSink)) {
	case "memory":
		return
	case "elasticsearch", "es":
		go pushElasticsearch(context.Background(), e)
	case "http", "webhook":
		go pushHTTP(context.Background(), e)
	case "loki":
		go pushLoki(context.Background(), e)
	case "docker_json", "promtail":
		// 单行 JSON → 进程 stdout → Docker json-file → Promtail → Loki（推荐生产路径）
		eout := e
		eout.LogStream = "lowcode_faas_function_execution"
		b, err := json.Marshal(eout)
		if err != nil {
			return
		}
		_, _ = fmt.Fprintln(os.Stdout, string(b))
	case "", "stdout":
		b, _ := json.Marshal(e)
		log.Printf("[function-log] %s", string(b))
	default:
		b, _ := json.Marshal(e)
		log.Printf("[function-log] %s", string(b))
	}
}
