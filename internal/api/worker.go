package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/monoposer/lowcode-faas/internal/jscache"
	"github.com/monoposer/lowcode-faas/internal/metaclient"
	"github.com/monoposer/lowcode-faas/internal/runner"
)

// WorkerHandler serves the public invoke API.
// It resolves actions via meta (with local LRU + If-None-Match), then runs qjs.
type WorkerHandler struct {
	runner *runner.Runner
	meta   *metaclient.Client
	cache  *jscache.LRU
	log    *slog.Logger
}

func NewWorkerHandler(r *runner.Runner, meta *metaclient.Client, cache *jscache.LRU, log *slog.Logger) *WorkerHandler {
	if cache == nil {
		cache = jscache.New(128)
	}
	if log == nil {
		log = slog.Default()
	}
	return &WorkerHandler{runner: r, meta: meta, cache: cache, log: log}
}

func (h *WorkerHandler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":     "ok",
			"role":       "worker",
			"cache_size": h.cache.Len(),
		})
	})
	mux.HandleFunc("POST /api/actions/{name}/invoke", h.invoke)
	return withCORS(mux)
}

func (h *WorkerHandler) invoke(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	runID := newRunID()
	name := r.PathValue("name")
	group := groupFromRequest(r)
	if group == "" {
		writeErr(w, http.StatusBadRequest, "group is required (query group, X-Tenant-Id, or X-Solution-Id)")
		return
	}
	log := h.log.With(
		"run_id", runID,
		"action", name,
		"group", group,
	)

	if h.meta == nil {
		log.Error("invoke rejected", "error", "meta client not configured")
		writeErr(w, http.StatusServiceUnavailable, "meta client not configured (set LOWCODE_FAAS_META_URL)")
		return
	}

	var body InvokeBody
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		log.Warn("invoke bad request", "error", "invalid json body")
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}

	log.Info("invoke start")

	rt, cacheStatus, err := h.resolveRuntime(r, name, group)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Warn("invoke not found", "error", err.Error(), "duration_ms", time.Since(start).Milliseconds())
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		log.Error("invoke meta resolve failed", "error", err.Error(), "duration_ms", time.Since(start).Milliseconds())
		writeErr(w, http.StatusBadGateway, "meta: "+err.Error())
		return
	}
	log = log.With("etag", rt.Etag, "cache", cacheStatus)
	log.Info("invoke resolved")

	timeout := time.Duration(rt.Timeout) * time.Second
	if body.TimeoutMs != nil && *body.TimeoutMs > 0 {
		timeout = time.Duration(*body.TimeoutMs) * time.Millisecond
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	req := buildActionRequest(body)

	res, err := h.runner.Invoke(r.Context(), []byte(rt.JS), req, timeout)
	if err != nil {
		log.Error("invoke runner error",
			"error", err.Error(),
			"timeout_ms", timeout.Milliseconds(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	status := "succeeded"
	errMsg := ""
	if res.Error != "" {
		status = "failed"
		errMsg = res.Error
	}
	out := normalizeActionResponse(res.Output)

	attrs := []any{
		"status", status,
		"http_status", out.Status,
		"exec_ms", res.Duration.Milliseconds(),
		"duration_ms", time.Since(start).Milliseconds(),
		"timeout_ms", timeout.Milliseconds(),
		"js_bytes", len(rt.JS),
	}
	if res.Logs != "" {
		attrs = append(attrs, "script_logs", res.Logs)
	}
	if errMsg != "" {
		attrs = append(attrs, "error", errMsg)
		log.Error("invoke finished", attrs...)
	} else {
		log.Info("invoke finished", attrs...)
	}

	httpCode := http.StatusOK
	if status == "failed" {
		httpCode = http.StatusInternalServerError
		out = &ActionResponseDTO{
			Status: httpCode,
			Data:   map[string]any{"error": errMsg},
		}
	} else {
		httpCode = clampHTTPStatus(out.Status)
		out.Status = httpCode
	}
	w.Header().Set("X-Faas-Run-Id", runID)
	w.Header().Set("X-Faas-Action", rt.Name)
	w.Header().Set("X-Faas-Etag", rt.Etag)
	w.Header().Set("X-Faas-Duration-Ms", strconv.FormatInt(res.Duration.Milliseconds(), 10))
	// Invoke HTTP body is only { status, data } — no logs key (script logs → worker slog only).
	writeJSON(w, httpCode, out)
}

// resolveRuntime returns the runtime action and a cache status: "hit" | "refresh" | "miss".
func (h *WorkerHandler) resolveRuntime(r *http.Request, name, group string) (*metaclient.RuntimeAction, string, error) {
	key := jscache.Key(group, name)
	cached, hit := h.cache.Get(key)
	ifNoneMatch := ""
	if hit {
		ifNoneMatch = cached.Etag
	}

	rt, notModified, err := h.meta.GetRuntime(r.Context(), name, group, ifNoneMatch)
	if err != nil {
		return nil, "", err
	}
	if notModified {
		if !hit {
			return nil, "", errors.New("meta returned 304 but cache miss")
		}
		return &metaclient.RuntimeAction{
			Name:    cached.Name,
			Group:   cached.Group,
			Etag:    cached.Etag,
			Timeout: cached.Timeout,
			JsURL:   cached.JsURL,
			JS:      cached.JS,
		}, "hit", nil
	}

	status := "miss"
	if hit {
		status = "refresh"
	}
	h.cache.Put(key, jscache.Entry{
		Name:    rt.Name,
		Group:   rt.Group,
		Etag:    rt.Etag,
		Timeout: rt.Timeout,
		JsURL:   rt.JsURL,
		JS:      rt.JS,
	})
	return rt, status, nil
}
