package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// handleFunctionWithID 处理 POST /functions/{name}/invoke（调用函数）
func handleFunctionWithID(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/functions/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || parts[1] != "invoke" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	name := parts[0]

	fn, err := getFunctionByName(r.Context(), name)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "function not found", http.StatusNotFound)
			return
		}
		log.Printf("query function error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req invokeFunctionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	timeout := 5 * time.Second
	if req.TimeoutMs > 0 {
		timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}

	result, err := runFunction(r.Context(), fn, req.Input, timeout)
	if err != nil {
		log.Printf("run function error: %v", err)
		http.Error(w, "internal error running function", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
