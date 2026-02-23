package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// runFunction 根据语言在 Docker 容器中执行并返回结果
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

	var cmd *exec.Cmd
	switch fn.Language {
	case "python":
		if err := writeFile(tmpDir, "main.py", []byte(fn.SourceCode)); err != nil {
			return nil, err
		}
		cmd = exec.CommandContext(ctx, "docker", "run", "--rm", "-i",
			"-v", fmt.Sprintf("%s:/workspace:ro", tmpDir),
			"-w", "/workspace",
			"python:3.11-slim", "python", "main.py")
	case "deno":
		if err := writeFile(tmpDir, "main.ts", []byte(fn.SourceCode)); err != nil {
			return nil, err
		}
		cmd = exec.CommandContext(ctx, "docker", "run", "--rm", "-i",
			"-v", fmt.Sprintf("%s:/workspace:ro", tmpDir),
			"-w", "/workspace",
			"denoland/deno:alpine", "deno", "run", "--quiet", "main.ts")
	default:
		return nil, fmt.Errorf("unsupported language: %s", fn.Language)
	}

	if len(input) > 0 && string(input) != "null" {
		cmd.Stdin = bytes.NewReader(input)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	runStatus := "success"
	exitCode := 0

	err = cmd.Run()
	duration := time.Since(start)

	if ctx.Err() == context.DeadlineExceeded {
		runStatus = "timeout"
		exitCode = -1
	} else if err != nil {
		runStatus = "failed"
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	} else {
		exitCode = 0
	}

	stdout := stdoutBuf.Bytes()
	stderr := strings.TrimSpace(stderrBuf.String())

	var outputJSON json.RawMessage
	if len(stdout) > 0 {
		var js json.RawMessage
		if json.Unmarshal(stdout, &js) == nil {
			outputJSON = js
		} else {
			wrapped, _ := json.Marshal(string(stdout))
			outputJSON = wrapped
		}
	}

	return &RunResult{
		RunID:        runID,
		Status:       runStatus,
		Output:       outputJSON,
		ErrorMessage: stderr,
		ExitCode:     exitCode,
		DurationMs:   duration.Milliseconds(),
		FunctionID:   fn.ID,
	}, nil
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
