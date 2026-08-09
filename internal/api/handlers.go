package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"lowcode-faas/internal/model"
	"lowcode-faas/internal/store"
	"lowcode-faas/internal/tscompile"
)

// actionNameRe: identifier used in invoke paths (matches playground).
var actionNameRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// Handler is the meta API: CRUD + TS→JS compile on save. Source + JS always land in S3 OSS.
// Public invoke lives on the worker; meta only exposes /runtime for the worker to fetch metadata+JS.
type Handler struct {
	db       *store.Postgres
	compiler *tscompile.Compiler
	uploader store.Uploader
}

func NewHandler(db *store.Postgres, compiler *tscompile.Compiler, uploader store.Uploader) *Handler {
	return &Handler{db: db, compiler: compiler, uploader: uploader}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /api/actions", h.list)
	mux.HandleFunc("GET /api/actions/{name}", h.get)
	mux.HandleFunc("GET /api/actions/{name}/runtime", h.runtime)
	mux.HandleFunc("POST /api/actions", h.create)
	mux.HandleFunc("PUT /api/actions/{id}", h.update)
	mux.HandleFunc("DELETE /api/actions/{id}", h.delete)
	mux.HandleFunc("POST /api/actions/{id}/compile", h.recompile)
	return withCORS(mux)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"ok": false, "error": msg})
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "role": "meta"})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.db.List(r.Context(), store.ListOpts{
		Group: strings.TrimSpace(r.URL.Query().Get("group")),
		Q:     strings.TrimSpace(r.URL.Query().Get("q")),
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": toDTOList(items)})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var (
		item *model.Action
		err  error
	)
	if g := strings.TrimSpace(r.URL.Query().Get("group")); g != "" {
		item, err = h.db.GetByGroupName(r.Context(), g, name)
	} else {
		item, err = h.db.GetByName(r.Context(), name)
	}
	if err != nil {
		writeNotFound(w, err)
		return
	}
	dto := toDTO(item)
	if src, err := h.resolveSource(r.Context(), item); err == nil {
		dto.Content = src
	}
	writeJSON(w, http.StatusOK, dto)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var body CreateBody
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if !actionNameRe.MatchString(name) {
		writeErr(w, http.StatusBadRequest, "name must start with a letter and use only letters, digits, _ or -")
		return
	}
	source := body.Content
	if strings.TrimSpace(source) == "" {
		source = tscompile.DefaultHandlerSource
	}
	label := strings.TrimSpace(body.Label)
	if label == "" {
		label = name
	}
	group := strings.TrimSpace(body.Group)
	sourceType := tscompile.NormalizeSourceType(body.SourceType)
	timeout := 60
	if body.Timeout != nil {
		timeout = *body.Timeout
	}
	async := false
	if body.Async != nil {
		async = *body.Async
	}

	etag, sourceURL, jsURL, err := h.compileAndUpload(r.Context(), name, sourceType, source)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	item, err := h.db.Create(r.Context(), model.CreateAction{
		Name:        name,
		Label:       label,
		Group:       group,
		Description: body.Description,
		SourceType:  sourceType,
		SourceURL:   sourceURL,
		JsURL:       jsURL,
		Etag:        etag,
		Async:       async,
		Timeout:     timeout,
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			writeErr(w, http.StatusConflict, "action name already exists in this group")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	dto := toDTO(item)
	dto.Content = source
	writeJSON(w, http.StatusCreated, dto)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body UpdateBody
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}
	existing, err := h.db.GetByID(r.Context(), id)
	if err != nil {
		writeNotFound(w, err)
		return
	}

	source := ""
	if body.Content != nil {
		source = *body.Content
	} else {
		source, err = h.resolveSource(r.Context(), existing)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "source unavailable: "+err.Error())
			return
		}
	}
	sourceType := existing.SourceType
	if body.SourceType != nil {
		sourceType = tscompile.NormalizeSourceType(*body.SourceType)
	} else {
		sourceType = tscompile.NormalizeSourceType(sourceType)
	}
	name := existing.Name
	if body.Name != nil && strings.TrimSpace(*body.Name) != "" {
		name = strings.TrimSpace(*body.Name)
	}

	etag, sourceURL, jsURL, err := h.compileAndUpload(r.Context(), name, sourceType, source)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	item, err := h.db.Update(r.Context(), id, model.UpdateAction{
		Name:        body.Name,
		Label:       body.Label,
		Group:       body.Group,
		Description: body.Description,
		Async:       body.Async,
		Timeout:     body.Timeout,
		SourceType:  &sourceType,
		Etag:        &etag,
		SourceURL:   &sourceURL,
		JsURL:       &jsURL,
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			writeErr(w, http.StatusConflict, "action name already exists in this group")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	dto := toDTO(item)
	dto.Content = source
	writeJSON(w, http.StatusOK, dto)
}

func (h *Handler) recompile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := h.db.GetByID(r.Context(), id)
	if err != nil {
		writeNotFound(w, err)
		return
	}
	source, err := h.resolveSource(r.Context(), existing)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "source unavailable: "+err.Error())
		return
	}
	st := tscompile.NormalizeSourceType(existing.SourceType)
	etag, sourceURL, jsURL, err := h.compileAndUpload(r.Context(), existing.Name, st, source)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := h.db.Update(r.Context(), id, model.UpdateAction{
		SourceType: &st,
		Etag:       &etag,
		SourceURL:  &sourceURL,
		JsURL:      &jsURL,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	dto := toDTO(item)
	dto.Content = source
	writeJSON(w, http.StatusOK, dto)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.db.SoftDelete(r.Context(), id); err != nil {
		writeNotFound(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// runtime returns metadata + compiled JS for the worker (no TypeScript source).
// Supports If-None-Match: when it equals the current etag, responds 304 without fetching OSS.
func (h *Handler) runtime(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var (
		item *model.Action
		err  error
	)
	if g := strings.TrimSpace(r.URL.Query().Get("group")); g != "" {
		item, err = h.db.GetByGroupName(r.Context(), g, name)
	} else {
		item, err = h.db.GetByName(r.Context(), name)
	}
	if err != nil {
		writeNotFound(w, err)
		return
	}
	if item.JsURL == "" {
		writeErr(w, http.StatusBadRequest, "action has no compiled js_url (save/compile first)")
		return
	}
	w.Header().Set("ETag", item.Etag)
	if inm := strings.TrimSpace(r.Header.Get("If-None-Match")); inm != "" && inm == item.Etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	jsBytes, err := h.uploader.GetBytes(r.Context(), item.JsURL)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "fetch js from oss: "+err.Error())
		return
	}
	if len(jsBytes) == 0 {
		writeErr(w, http.StatusBadRequest, "empty js artifact in oss")
		return
	}
	writeJSON(w, http.StatusOK, RuntimeActionDTO{
		Name:    item.Name,
		Group:   item.Group,
		Etag:    item.Etag,
		Timeout: item.Timeout,
		JsURL:   item.JsURL,
		JS:      string(jsBytes),
	})
}

// compileAndUpload always writes both .ts and .js into S3 OSS and returns their URLs.
func (h *Handler) compileAndUpload(ctx context.Context, name, sourceType, source string) (etag, sourceURL, jsURL string, err error) {
	st := tscompile.NormalizeSourceType(sourceType)
	etag = tscompile.HashSource(st, source)
	if h.uploader == nil {
		return "", "", "", errors.New("oss uploader not configured")
	}
	if h.compiler == nil {
		return "", "", "", errors.New("ts compiler not configured")
	}
	sourceURL, err = h.uploader.Put(ctx, tscompile.SourceKey(name, etag), strings.NewReader(source))
	if err != nil {
		return "", "", "", fmt.Errorf("upload source: %w", err)
	}
	bin, err := h.compiler.Compile(ctx, st, source)
	if err != nil {
		return "", "", "", err
	}
	if len(bin) == 0 {
		return "", "", "", errors.New("compiler produced empty js")
	}
	jsURL, err = h.uploader.Put(ctx, tscompile.ArtifactKey(name, etag), bytes.NewReader(bin))
	if err != nil {
		return "", "", "", fmt.Errorf("upload js: %w", err)
	}
	if sourceURL == "" || jsURL == "" {
		return "", "", "", errors.New("source_url and js_url are required in oss")
	}
	return etag, sourceURL, jsURL, nil
}

func (h *Handler) resolveSource(ctx context.Context, a *model.Action) (string, error) {
	if a == nil {
		return "", errors.New("action is nil")
	}
	if a.SourceURL != "" && h.uploader != nil {
		b, err := h.uploader.GetBytes(ctx, a.SourceURL)
		if err == nil {
			return string(b), nil
		}
		return "", err
	}
	return "", errors.New("no source_url")
}

func writeNotFound(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeErr(w, http.StatusInternalServerError, err.Error())
}

func newRunID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
