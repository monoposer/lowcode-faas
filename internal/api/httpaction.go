package api

import (
	"encoding/json"
	"strings"
)

// buildActionRequest maps invoke JSON into the ActionRequest object passed to the handler.
func buildActionRequest(body InvokeBody) map[string]any {
	if body.Input != nil {
		switch v := body.Input.(type) {
		case map[string]any:
			return withRequestDefaults(v)
		default:
			return withRequestDefaults(map[string]any{"body": v})
		}
	}

	ctx := body.Context
	if ctx == nil {
		ctx = map[string]any{}
	}
	data := body.Data
	if data == nil {
		data = map[string]any{}
	}
	query := body.Query
	if query == nil {
		query = map[string]string{}
	}
	headers := body.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	method := strings.TrimSpace(body.Method)
	if method == "" {
		method = "POST"
	}
	return map[string]any{
		"context": ctx,
		"body":    body.Body,
		"data":    data,
		"query":   query,
		"method":  method,
		"headers": headers,
		"path":    body.Path,
	}
}

func withRequestDefaults(m map[string]any) map[string]any {
	out := make(map[string]any, len(m)+6)
	for k, v := range m {
		out[k] = v
	}
	// Prefer `context`; accept legacy `ctx`.
	if _, ok := out["context"]; !ok {
		if c, ok := out["ctx"]; ok {
			out["context"] = c
		} else {
			out["context"] = map[string]any{}
		}
	}
	if _, ok := out["data"]; !ok {
		out["data"] = map[string]any{}
	}
	if _, ok := out["query"]; !ok {
		out["query"] = map[string]string{}
	}
	if _, ok := out["headers"]; !ok {
		out["headers"] = map[string]string{}
	}
	if method, _ := out["method"].(string); strings.TrimSpace(method) == "" {
		out["method"] = "POST"
	}
	if _, ok := out["path"]; !ok {
		out["path"] = ""
	}
	if _, ok := out["body"]; !ok {
		out["body"] = nil
	}
	return out
}

// normalizeActionResponse coerces a handler return value into { status, data }.
// Recognizes Response shapes with status/data (and legacy body→data).
// Anything else becomes { status: 200, data: value }.
func normalizeActionResponse(out any) *ActionResponseDTO {
	if out == nil {
		return &ActionResponseDTO{Status: 200, Data: nil}
	}
	m, ok := out.(map[string]any)
	if !ok {
		return &ActionResponseDTO{Status: 200, Data: out}
	}
	_, hasStatus := m["status"]
	_, hasData := m["data"]
	_, hasBody := m["body"]
	_, hasHeaders := m["headers"]
	if !hasStatus && !hasData && !hasBody && !hasHeaders {
		return &ActionResponseDTO{Status: 200, Data: m}
	}

	dto := &ActionResponseDTO{Status: 200, Data: nil}
	if hasStatus {
		dto.Status = asHTTPStatus(m["status"])
	}
	switch {
	case hasData:
		dto.Data = m["data"]
	case hasBody:
		dto.Data = m["body"]
	}
	return dto
}

func asHTTPStatus(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case float32:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 200
	}
}

func clampHTTPStatus(code int) int {
	if code < 100 || code > 599 {
		return 200
	}
	return code
}
