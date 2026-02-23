package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// handleFunctions 处理 POST /functions（创建函数）
func handleFunctions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req createFunctionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	req.Language = strings.ToLower(strings.TrimSpace(req.Language))
	if req.Language != "python" && req.Language != "deno" {
		http.Error(w, "language must be 'python' or 'deno'", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.SourceCode) == "" {
		http.Error(w, "source_code is required", http.StatusBadRequest)
		return
	}

	fn, err := createFunction(r.Context(), req.Name, req.Language, req.SourceCode)
	if err != nil {
		log.Printf("create function error: %v", err)
		http.Error(w, "create function failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(fn)
}
