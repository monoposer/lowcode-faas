package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"lowcode-faas/internal/envutil"
	"lowcode-faas/internal/model"
	"lowcode-faas/internal/store"
)

func handleCollectionsRoot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		prefix := strings.TrimSpace(r.URL.Query().Get("prefix"))
		items, err := store.ListCollections(r.Context(), prefix)
		if err != nil {
			log.Printf("list collections error: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if items == nil {
			items = []*model.CollectionListItem{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(items)
	case http.MethodPost:
		var req model.CreateCollectionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		item, err := store.CreateCollection(r.Context(), req.Path, req.Env)
		if err != nil {
			if errors.Is(err, store.ErrCollectionExists) {
				http.Error(w, "collection already exists", http.StatusConflict)
				return
			}
			log.Printf("create collection error: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(item)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleCollectionsSlash(w http.ResponseWriter, r *http.Request) {
	raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/collections/"), "/")
	if raw == "" {
		http.NotFound(w, r)
		return
	}

	// .../{collection}/env 或 collections/env（根集合）
	var envCol string
	var isEnv bool
	if raw == "env" {
		envCol = ""
		isEnv = true
	} else if strings.HasSuffix(raw, "/env") {
		envCol = strings.Trim(strings.TrimSuffix(raw, "/env"), "/")
		isEnv = true
	}
	if isEnv {
		cp, err := store.NormalizeCollectionPath(envCol)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		handleCollectionEnv(w, r, cp)
		return
	}

	idx := strings.Index(raw, "/functions/")
	if idx >= 0 {
		colPart := strings.Trim(raw[:idx], "/")
		tail := strings.Trim(raw[idx+len("/functions/"):], "/")
		cp, err := store.NormalizeCollectionPath(colPart)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if tail == "" {
			switch r.Method {
			case http.MethodGet:
				handleListFunctions(w, r, cp)
			case http.MethodPost:
				handleCreate(w, r, cp)
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		parts := pathParts(tail)
		if len(parts) == 1 {
			name := parts[0]
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
		if len(parts) == 2 {
			name, action := parts[0], parts[1]
			switch action {
			case "invoke":
				handleInvoke(w, r, cp, name)
			case "pull":
				handlePull(w, r, cp, name)
			case "versions":
				if r.Method != http.MethodGet {
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
					return
				}
				handleListVersions(w, r, cp, name)
			default:
				http.NotFound(w, r)
			}
			return
		}
		http.NotFound(w, r)
		return
	}

	if strings.HasSuffix(raw, "/functions") {
		col := strings.Trim(strings.TrimSuffix(raw, "/functions"), "/")
		cp, err := store.NormalizeCollectionPath(col)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodGet:
			handleListFunctions(w, r, cp)
		case http.MethodPost:
			handleCreate(w, r, cp)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	http.NotFound(w, r)
}

func handleCollectionEnv(w http.ResponseWriter, r *http.Request, collectionPath string) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Env map[string]string `json:"env"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if err := envutil.ValidateEnvMap(body.Env); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := store.SetCollectionEnv(r.Context(), collectionPath, body.Env); err != nil {
		if errors.Is(err, store.ErrCollectionNotFound) {
			http.Error(w, "collection not found", http.StatusNotFound)
			return
		}
		log.Printf("set collection env: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleListVersions(w http.ResponseWriter, r *http.Request, collectionPath, name string) {
	rows, err := store.ListFunctionVersions(r.Context(), collectionPath, name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "function not found", http.StatusNotFound)
			return
		}
		log.Printf("list versions: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rows)
}

func pathParts(rest string) []string {
	var parts []string
	for _, p := range strings.Split(rest, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func loadFunctionForInvoke(ctx context.Context, collectionPath, name string, version *int) (*model.Function, error) {
	fn, err := store.GetFunctionByName(ctx, collectionPath, name, version)
	if err != nil {
		return nil, err
	}
	chain, err := store.MergedAncestorCollectionEnvs(ctx, collectionPath)
	if err != nil {
		return nil, err
	}
	out := envutil.CloneFunction(fn)
	out.Env = envutil.MergeEnv(chain, out.Env)
	return out, nil
}
