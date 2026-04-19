package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lowcode-faas/internal/config"
	"lowcode-faas/internal/envutil"
	"lowcode-faas/internal/model"
	"lowcode-faas/internal/runner"
	"lowcode-faas/internal/sourcefetch"
	"lowcode-faas/internal/store"
	"lowcode-faas/internal/workerclient"
)

// Register 注册 HTTP 路由（调度侧 API）
func Register(mux *http.ServeMux) {
	mux.HandleFunc("/collections/", handleCollectionsSlash)
	mux.HandleFunc("/collections", handleCollectionsRoot)
	mux.HandleFunc("/functions", handleFunctionsCollection)
	mux.HandleFunc("/functions/", handleFunctionsPrefix)
	mux.HandleFunc("/function-logs/query", handleFunctionLogsQuery)
	mux.HandleFunc("/function-logs", handleFunctionLogs)
}

// effectiveCollectionPathFromRequest 嵌套路由 fixed 优先，否则读 ?collection_path=
func effectiveCollectionPathFromRequest(r *http.Request, fixed string) (string, error) {
	if strings.TrimSpace(fixed) != "" {
		return store.NormalizeCollectionPath(fixed)
	}
	return store.NormalizeCollectionPath(r.URL.Query().Get("collection_path"))
}

func effectiveCollectionPathForCreate(r *http.Request, fixed, fromBody string) (string, error) {
	if strings.TrimSpace(fixed) != "" {
		return store.NormalizeCollectionPath(fixed)
	}
	if strings.TrimSpace(fromBody) != "" {
		return store.NormalizeCollectionPath(fromBody)
	}
	return store.NormalizeCollectionPath(r.URL.Query().Get("collection_path"))
}

func handleFunctionsCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		handleListFunctions(w, r, "")
	case http.MethodPost:
		handleCreate(w, r, "")
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleListFunctions(w http.ResponseWriter, r *http.Request, fixedCollection string) {
	cp, err := effectiveCollectionPathFromRequest(r, fixedCollection)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	items, err := store.ListFunctions(r.Context(), cp)
	if err != nil {
		if errors.Is(err, store.ErrCollectionNotFound) {
			http.Error(w, "collection not found", http.StatusNotFound)
			return
		}
		log.Printf("list functions error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if items == nil {
		items = []*model.FunctionListItem{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}

func handleCreate(w http.ResponseWriter, r *http.Request, fixedCollection string) {
	var req model.CreateFunctionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	cp, err := effectiveCollectionPathForCreate(r, fixedCollection, req.CollectionPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.Language = strings.ToLower(strings.TrimSpace(req.Language))
	if req.Language == "" {
		req.Language = model.LangJavaScript
	}
	if req.Language != model.LangJavaScript {
		http.Error(w, `language must be "javascript"`, http.StatusBadRequest)
		return
	}
	source := strings.TrimSpace(req.SourceCode)
	if u := strings.TrimSpace(req.SourceURL); u != "" {
		code, err := sourcefetch.FetchSourceFromURL(r.Context(), u)
		if err != nil {
			log.Printf("create fetch source_url error: %v", err)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		source = code
	}
	if source == "" {
		http.Error(w, "source_code or source_url is required", http.StatusBadRequest)
		return
	}
	if err := envutil.ValidateEnvMap(req.Env); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fn, err := store.CreateFunction(r.Context(), cp, req.Name, req.Language, source, req.Env)
	if err != nil {
		if errors.Is(err, store.ErrUnsupportedLanguage) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if errors.Is(err, store.ErrCollectionNotFound) {
			http.Error(w, "collection not found", http.StatusNotFound)
			return
		}
		log.Printf("create function error: %v", err)
		http.Error(w, "create function failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(fn)
}

func handleFunctionsPrefix(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/functions/"), "/")
	var parts []string
	for _, p := range strings.Split(rest, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		http.NotFound(w, r)
		return
	}
	name := parts[0]
	cp, err := effectiveCollectionPathFromRequest(r, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			handleGetFunction(w, r, cp, name)
		case http.MethodPut:
			handleUpdateFunction(w, r, cp, name)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	if len(parts) == 2 && parts[1] == "versions" {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleListVersions(w, r, cp, name)
		return
	}
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	switch parts[1] {
	case "invoke":
		handleInvoke(w, r, cp, name)
	case "pull":
		handlePull(w, r, cp, name)
	default:
		http.NotFound(w, r)
	}
}

func queryVersionPtr(r *http.Request) *int {
	v := strings.TrimSpace(r.URL.Query().Get("version"))
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return nil
	}
	return &n
}

func invokeVersionPtr(req *model.InvokeFunctionRequest, r *http.Request) *int {
	if req.Version != nil {
		return req.Version
	}
	return queryVersionPtr(r)
}

func handleGetFunction(w http.ResponseWriter, r *http.Request, collectionPath, name string) {
	fn, err := store.GetFunctionByName(r.Context(), collectionPath, name, queryVersionPtr(r))
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
		log.Printf("get function error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(fn)
}

func handleUpdateFunction(w http.ResponseWriter, r *http.Request, collectionPath, name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		http.Error(w, "invalid name", http.StatusBadRequest)
		return
	}
	fn, err := store.GetFunctionByName(r.Context(), collectionPath, name, nil)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "function not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, store.ErrCollectionNotFound) {
			http.Error(w, "collection not found", http.StatusNotFound)
			return
		}
		log.Printf("get function for update error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	var req model.UpdateFunctionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	source := strings.TrimSpace(req.SourceCode)
	if source == "" {
		http.Error(w, "source_code is required", http.StatusBadRequest)
		return
	}
	env := fn.Env
	if req.Env != nil {
		env = envutil.CloneStringMap(*req.Env)
	}
	if err := envutil.ValidateEnvMap(env); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	updated, err := store.CreateFunction(r.Context(), collectionPath, name, fn.Language, source, env)
	if err != nil {
		if errors.Is(err, store.ErrUnsupportedLanguage) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if errors.Is(err, store.ErrCollectionNotFound) {
			http.Error(w, "collection not found", http.StatusNotFound)
			return
		}
		log.Printf("update function error: %v", err)
		http.Error(w, "update function failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
}

func handleInvoke(w http.ResponseWriter, r *http.Request, collectionPath, name string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req model.InvokeFunctionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if err := envutil.ValidateEnvMap(req.Env); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ver := invokeVersionPtr(&req, r)
	fn, err := loadFunctionForInvoke(r.Context(), collectionPath, name, ver)
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
		log.Printf("query function error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	timeout := config.DefaultInvokeTimeout
	if req.TimeoutMs > 0 {
		timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}
	var result *model.RunResult
	if u := strings.TrimSpace(config.WorkerBaseURL); u != "" {
		result, err = workerclient.Run(r.Context(), u, collectionPath, name, ver, req.Input, req.Env, timeout)
	} else {
		result, err = runner.RunJS(r.Context(), fn, req.Input, req.Env, timeout)
	}
	if err != nil {
		log.Printf("run function error: %v", err)
		http.Error(w, "internal error running function", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func handlePull(w http.ResponseWriter, r *http.Request, collectionPath, name string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	fn, err := store.GetFunctionByName(r.Context(), collectionPath, name, nil)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "function not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, store.ErrCollectionNotFound) {
			http.Error(w, "collection not found", http.StatusNotFound)
			return
		}
		log.Printf("query function error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	var req model.PullFunctionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	urlTrim := strings.TrimSpace(req.SourceURL)
	if urlTrim == "" {
		http.Error(w, "source_url is required", http.StatusBadRequest)
		return
	}
	source, err := sourcefetch.FetchSourceFromURL(r.Context(), urlTrim)
	if err != nil {
		log.Printf("pull source_url error: %v", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	updated, err := store.CreateFunction(r.Context(), collectionPath, name, fn.Language, source, fn.Env)
	if err != nil {
		log.Printf("pull save function error: %v", err)
		http.Error(w, "save function failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
}
