package model

import "time"

// Action is the persisted metadata row (TypeScript source + compiled JS live in blob storage).
type Action struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Label       string     `json:"label"`
	Group       string     `json:"group"`
	Description string     `json:"description"`
	SourceType  string     `json:"source_type"`
	SourceURL   string     `json:"source_url"`
	JsURL       string     `json:"js_url"`
	Etag        string     `json:"etag"`
	Async       bool       `json:"async"`
	Timeout     int        `json:"timeout"`
	Version     int64      `json:"version"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

// CreateAction is the input for inserting a new action row.
type CreateAction struct {
	Name        string
	Label       string
	Group       string
	Description string
	SourceType  string
	SourceURL   string
	JsURL       string
	Etag        string
	Async       bool
	Timeout     int
}

// UpdateAction is a partial update; nil pointer fields are left unchanged.
type UpdateAction struct {
	Name        *string
	Label       *string
	Group       *string
	Description *string
	SourceType  *string
	SourceURL   *string
	JsURL       *string
	Etag        *string
	Async       *bool
	Timeout     *int
}
