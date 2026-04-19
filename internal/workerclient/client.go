package workerclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"lowcode-faas/internal/model"
)

// Run 将执行任务转发给 worker HTTP 服务
func Run(ctx context.Context, baseURL, collectionPath, functionName string, version *int, input json.RawMessage, env map[string]string, timeout time.Duration) (*model.RunResult, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("worker base url is empty")
	}
	body, err := json.Marshal(model.WorkerRunRequest{
		CollectionPath: collectionPath,
		FunctionName:   functionName,
		Input:          input,
		TimeoutMs:      int(timeout / time.Millisecond),
		Env:            env,
		Version:        version,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/run", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: timeout + 5*time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("worker HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var out model.RunResult
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("decode worker response: %w", err)
	}
	return &out, nil
}
