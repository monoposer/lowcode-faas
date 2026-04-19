package functionlogs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lowcode-faas/internal/config"
)

func pushElasticsearch(ctx context.Context, e Entry) {
	base := strings.TrimRight(strings.TrimSpace(config.FunctionLogsESURL), "/")
	if base == "" {
		return
	}
	index := strings.TrimSpace(config.FunctionLogsESIndex)
	if index == "" {
		index = "lowcode-faas-logs"
	}
	u := base + "/" + index + "/_doc"
	body, err := json.Marshal(e)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if u := strings.TrimSpace(config.FunctionLogsESUser); u != "" {
		p := strings.TrimSpace(config.FunctionLogsESPassword)
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(u+":"+p)))
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("functionlogs elasticsearch: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		var buf [512]byte
		n, _ := resp.Body.Read(buf[:])
		log.Printf("functionlogs elasticsearch HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(buf[:n])))
	}
}

func pushLoki(ctx context.Context, e Entry) {
	base := strings.TrimRight(strings.TrimSpace(config.FunctionLogsLokiURL), "/")
	if base == "" {
		return
	}
	u := base + "/loki/api/v1/push"
	ts := strconv.FormatInt(e.TS.UnixNano(), 10)
	line, err := json.Marshal(e)
	if err != nil {
		return
	}
	coll := e.CollectionPath
	if coll == "" {
		coll = "_"
	}
	stream := map[string]string{
		"job":             "lowcode-faas-function",
		"function":        e.FunctionName,
		"collection_path": coll,
		"level":           e.Level,
	}
	payload := map[string]any{
		"streams": []map[string]any{{
			"stream": stream,
			"values": [][]string{{ts, string(line)}},
		}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if t := strings.TrimSpace(config.FunctionLogsLokiTenant); t != "" {
		req.Header.Set("X-Scope-OrgID", t)
	}
	client := &http.Client{Timeout: config.FunctionLogsLokiTimeout}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("functionlogs loki: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		var buf [512]byte
		n, _ := resp.Body.Read(buf[:])
		log.Printf("functionlogs loki HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(buf[:n])))
	}
}

func pushHTTP(ctx context.Context, e Entry) {
	u := strings.TrimSpace(config.FunctionLogsHTTPURL)
	if u == "" {
		return
	}
	payload := map[string]any{
		"entry": e,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if h := strings.TrimSpace(config.FunctionLogsHTTPHeader); h != "" {
		// 单条可选头 "Name: Value"
		if i := strings.IndexByte(h, ':'); i > 0 {
			req.Header.Set(strings.TrimSpace(h[:i]), strings.TrimSpace(h[i+1:]))
		}
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("functionlogs http sink: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("functionlogs http sink: HTTP %d", resp.StatusCode)
	}
}
