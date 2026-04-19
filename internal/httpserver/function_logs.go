package httpserver

import (
	"io"
	"net/http"
	"strings"
	"time"

	"lowcode-faas/internal/config"
	"lowcode-faas/internal/functionlogs"
)

func handleFunctionLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if u := strings.TrimSpace(config.WorkerBaseURL); u != "" {
		proxyWorkerFunctionLogs(w, r, u)
		return
	}
	functionlogs.ServeHTTPQuery(w, r)
}

func handleFunctionLogsQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if u := strings.TrimSpace(config.WorkerBaseURL); u != "" {
		proxyWorkerFunctionLogsQuery(w, r, u)
		return
	}
	functionlogs.ServeHTTPQueryHints(w, r)
}

func proxyWorkerFunctionLogsQuery(w http.ResponseWriter, r *http.Request, base string) {
	base = strings.TrimRight(base, "/")
	u := base + "/v1/function-logs/query?" + r.URL.RawQuery
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u, nil)
	if err != nil {
		http.Error(w, "bad request", http.StatusInternalServerError)
		return
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for k, vs := range resp.Header {
		if strings.EqualFold(k, "Content-Length") {
			continue
		}
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func proxyWorkerFunctionLogs(w http.ResponseWriter, r *http.Request, base string) {
	base = strings.TrimRight(base, "/")
	u := base + "/v1/function-logs?" + r.URL.RawQuery
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u, nil)
	if err != nil {
		http.Error(w, "bad request", http.StatusInternalServerError)
		return
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for k, vs := range resp.Header {
		if strings.EqualFold(k, "Content-Length") {
			continue
		}
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
