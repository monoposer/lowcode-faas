package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ensureDataDir 确保 data/functions 目录存在
func ensureDataDir() error {
	path := filepath.Join(dataDir, "functions")
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create data dir %s: %w", path, err)
	}
	return nil
}

// createFunction 在 data/functions 下创建函数文件，仅保存源码本身
func createFunction(ctx context.Context, name, language, source string) (*Function, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("function name is required")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return nil, fmt.Errorf("invalid function name")
	}

	id := name
	now := time.Now().UTC()

	var ext string
	switch language {
	case "python":
		ext = ".py"
	case "deno":
		ext = ".ts"
	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	fn := &Function{
		ID:         id,
		Name:       name,
		Language:   language,
		SourceCode: source,
		CreatedAt:  now,
	}

	path := filepath.Join(dataDir, "functions", id+ext)
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		return nil, fmt.Errorf("write function file: %w", err)
	}
	return fn, nil
}

// getFunctionByName 根据名称从 data/functions 读取函数，自动根据扩展名判定语言
func getFunctionByName(ctx context.Context, name string) (*Function, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrNotFound
	}

	candidates := []struct {
		ext      string
		language string
	}{
		{".py", "python"},
		{".ts", "deno"},
	}

	for _, c := range candidates {
		path := filepath.Join(dataDir, "functions", name+c.ext)
		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		code, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return &Function{
			ID:         name,
			Name:       name,
			Language:   c.language,
			SourceCode: string(code),
			CreatedAt:  info.ModTime(),
		}, nil
	}

	return nil, ErrNotFound
}
