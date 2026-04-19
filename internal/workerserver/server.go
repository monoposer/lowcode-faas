package workerserver

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"lowcode-faas/internal/config"
	"lowcode-faas/internal/envutil"
	"lowcode-faas/internal/functionlogs"
	"lowcode-faas/internal/model"
	"lowcode-faas/internal/runner"
	"lowcode-faas/internal/store"
)

// Register worker HTTP：仅接受调度进程转发的执行请求
func Register(mux *http.ServeMux) {
	mux.HandleFunc("/v1/run", handleRun)
	mux.HandleFunc("/v1/function-logs/query", functionlogs.ServeHTTPQueryHints)
	mux.HandleFunc("/v1/function-logs", functionlogs.ServeHTTPQuery)
}

func handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req model.WorkerRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if err := envutil.ValidateEnvMap(req.Env); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	collectionPath, err := store.NormalizeCollectionPath(req.CollectionPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fn, err := store.GetFunctionByName(r.Context(), collectionPath, req.FunctionName, req.Version)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "function not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, store.ErrUnsupportedVersionPin) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if errors.Is(err, store.ErrCollectionNotFound) {
			http.Error(w, "collection not found", http.StatusNotFound)
			return
		}
		log.Printf("worker load function: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	chain, err := store.MergedAncestorCollectionEnvs(r.Context(), collectionPath)
	if err != nil {
		log.Printf("worker collection env chain: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	runFn := envutil.CloneFunction(fn)
	runFn.Env = envutil.MergeEnv(chain, runFn.Env)
	timeout := config.DefaultInvokeTimeout
	if req.TimeoutMs > 0 {
		timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}
	result, err := runner.RunJS(r.Context(), runFn, req.Input, req.Env, timeout)
	if err != nil {
		log.Printf("worker run: %v", err)
		http.Error(w, "internal error running function", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// LoggingMiddleware 访问日志
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
