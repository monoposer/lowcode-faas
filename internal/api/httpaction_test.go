package api

import (
	"testing"
)

func TestNormalizeActionResponseHTTPShape(t *testing.T) {
	out := normalizeActionResponse(map[string]any{
		"status": 201,
		"data":   map[string]any{"id": "x"},
	})
	if out.Status != 201 {
		t.Fatalf("status: %d", out.Status)
	}
	data, _ := out.Data.(map[string]any)
	if data["id"] != "x" {
		t.Fatalf("data: %#v", out.Data)
	}
}

func TestNormalizeActionResponseLegacyBody(t *testing.T) {
	out := normalizeActionResponse(map[string]any{
		"status": 200,
		"body":   map[string]any{"ok": true},
	})
	data, _ := out.Data.(map[string]any)
	if data["ok"] != true {
		t.Fatalf("body→data: %#v", out.Data)
	}
}

func TestNormalizeActionResponseWrapBare(t *testing.T) {
	out := normalizeActionResponse(map[string]any{"ok": true})
	if out.Status != 200 {
		t.Fatalf("status: %d", out.Status)
	}
	data, _ := out.Data.(map[string]any)
	if data["ok"] != true {
		t.Fatalf("wrapped data: %#v", out.Data)
	}
}

func TestBuildActionRequestFlat(t *testing.T) {
	req := buildActionRequest(InvokeBody{
		Context: map[string]any{"user": "a"},
		Body:    map[string]any{"n": 1},
		Data:    map[string]any{"id": "9"},
		Query:   map[string]string{"q": "1"},
		Method:  "GET",
		Path:    "/x",
	})
	if req["method"] != "GET" || req["path"] != "/x" {
		t.Fatalf("%#v", req)
	}
	if req["context"].(map[string]any)["user"] != "a" {
		t.Fatalf("context: %#v", req["context"])
	}
}

func TestBuildActionRequestLegacyCtx(t *testing.T) {
	req := buildActionRequest(InvokeBody{
		Input: map[string]any{
			"ctx":  map[string]any{"legacy": true},
			"body": map[string]any{"x": 1},
		},
	})
	ctx := req["context"].(map[string]any)
	if ctx["legacy"] != true {
		t.Fatalf("expected ctx→context: %#v", req)
	}
}
