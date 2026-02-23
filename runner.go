package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// runFunction 启动容器内 HTTP 服务，转发请求并返回其 HTTP 响应；容器内 print/log 输出到容器控制台
func runFunction(ctx context.Context, fn *Function, input json.RawMessage, timeout time.Duration) (*RunResult, error) {
	runID := newID("run")
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "lowcode-faas-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	hostPort, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("get free port: %w", err)
	}

	var cmd *exec.Cmd
	switch fn.Language {
	case "python":
		if err := writeFile(tmpDir, "main.py", []byte(fn.SourceCode)); err != nil {
			return nil, err
		}
		cmd = exec.CommandContext(ctx, "docker", "run", "--rm",
			"-p", fmt.Sprintf("%d:%d", hostPort, ContainerHTTPPort),
			"-e", fmt.Sprintf("PORT=%d", ContainerHTTPPort),
			"-v", fmt.Sprintf("%s:/workspace:ro", tmpDir),
			"-w", "/workspace",
			PythonDockerImage, "python", "main.py")
	case "deno":
		if err := writeFile(tmpDir, "main.ts", []byte(fn.SourceCode)); err != nil {
			return nil, err
		}
		cmd = exec.CommandContext(ctx, "docker", "run", "--rm",
			"-p", fmt.Sprintf("%d:%d", hostPort, ContainerHTTPPort),
			"-e", fmt.Sprintf("PORT=%d", ContainerHTTPPort),
			"-v", fmt.Sprintf("%s:/workspace:ro", tmpDir),
			"-w", "/workspace",
			DenoDockerImage, "deno", "run", "--allow-net", "--quiet", "main.ts")
	default:
		return nil, fmt.Errorf("unsupported language: %s", fn.Language)
	}
	// 容器内 stdout/stderr 直接输出到宿主机控制台，不捕获
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start container: %w", err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", hostPort)
	if err := waitServerReady(ctx, baseURL); err != nil {
		duration := time.Since(start)
		return &RunResult{
			RunID:        runID,
			Status:       "failed",
			ErrorMessage: err.Error(),
			DurationMs:   duration.Milliseconds(),
			FunctionID:   fn.ID,
		}, nil
	}

	reqBody := input
	if len(reqBody) == 0 || string(reqBody) == "null" {
		reqBody = []byte("{}")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/", strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: timeout - time.Since(start)}
	if client.Timeout < time.Second {
		client.Timeout = time.Second
	}
	resp, err := client.Do(req)
	duration := time.Since(start)
	if err != nil {
		return &RunResult{
			RunID:        runID,
			Status:       "failed",
			ErrorMessage: err.Error(),
			DurationMs:   duration.Milliseconds(),
			FunctionID:   fn.ID,
		}, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	var outputJSON json.RawMessage
	if len(body) > 0 {
		var js json.RawMessage
		if json.Unmarshal(body, &js) == nil {
			outputJSON = js
		} else {
			wrapped, _ := json.Marshal(string(body))
			outputJSON = wrapped
		}
	}

	return &RunResult{
		RunID:          runID,
		Status:         "success",
		HTTPStatusCode: resp.StatusCode,
		HTTPHeaders:    headers,
		Output:         outputJSON,
		DurationMs:     duration.Milliseconds(),
		FunctionID:     fn.ID,
	}, nil
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitServerReady(ctx context.Context, baseURL string) error {
	deadline := time.Now().Add(ServerReadyWait * time.Second)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/", nil)
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			return nil // 能连上即视为就绪（GET 可能 404，无妨）
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("server not ready within %ds", ServerReadyWait)
}

func writeFile(dir, name string, data []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}
	return nil
}

func saveRun(ctx context.Context, r *RunResult) error {
	return nil
}
