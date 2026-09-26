package api

import (
	"github.com/monoposer/lowcode-faas/internal/model"
	"time"
)

// ActionDTO is the HTTP-facing shape for Action Studio / playground.
type ActionDTO struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Label       string `json:"label"`
	Group       string `json:"group"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content,omitempty"`
	SourceType  string `json:"source_type"`
	SourceURL   string `json:"source_url,omitempty"`
	JsURL       string `json:"js_url,omitempty"`
	Async       bool   `json:"async"`
	Timeout     int    `json:"timeout"`
	Version     int64  `json:"version"`
	Etag        string `json:"etag,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

type CreateBody struct {
	Name        string `json:"name"`
	Group       string `json:"group"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Content     string `json:"content"` // optional; default TypeScript template when empty
	SourceType  string `json:"source_type"`
	Async       *bool  `json:"async"`
	Timeout     *int   `json:"timeout"`
}

type UpdateBody struct {
	Name        *string `json:"name"`
	Group       *string `json:"group"`
	Content     *string `json:"content"`
	Label       *string `json:"label"`
	Description *string `json:"description"`
	Async       *bool   `json:"async"`
	Timeout     *int    `json:"timeout"`
	SourceType  *string `json:"source_type"`
}

type InvokeBody struct {
	// HTTP-mode request fields (preferred).
	Context map[string]any    `json:"context"`
	Body    any               `json:"body"`
	Data    map[string]any    `json:"data"`
	Query   map[string]string `json:"query"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Path    string            `json:"path"`

	// Input is a legacy full ActionRequest object (or partial). Prefer flat fields above.
	Input any `json:"input"`

	TimeoutMs *int `json:"timeout_ms"`
}

// ActionResponseDTO is the logical HTTP response from an action handler
// and is written as the invoke HTTP JSON body: { status, data }.
type ActionResponseDTO struct {
	Status int `json:"status"`
	Data   any `json:"data,omitempty"`
}

// RuntimeActionDTO is returned by GET /api/actions/{name}/runtime for the worker.
type RuntimeActionDTO struct {
	Name    string `json:"name"`
	Group   string `json:"group"`
	Etag    string `json:"etag"`
	Timeout int    `json:"timeout"`
	JsURL   string `json:"js_url"`
	JS      string `json:"js"`
}

func toDTO(a *model.Action) *ActionDTO {
	if a == nil {
		return nil
	}
	return &ActionDTO{
		ID:          a.ID,
		Name:        a.Name,
		Label:       a.Label,
		Group:       a.Group,
		Description: a.Description,
		SourceType:  a.SourceType,
		SourceURL:   a.SourceURL,
		JsURL:       a.JsURL,
		Async:       a.Async,
		Timeout:     a.Timeout,
		Version:     a.Version,
		Etag:        a.Etag,
		CreatedAt:   a.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   a.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toDTOList(items []*model.Action) []*ActionDTO {
	out := make([]*ActionDTO, 0, len(items))
	for _, it := range items {
		out = append(out, toDTO(it))
	}
	return out
}
