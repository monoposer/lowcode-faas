package tscompile

import (
	"fmt"
	"strings"
)

// ArtifactKey is the S3 object key suffix for compiled JS (prefixed with action-js/ by the uploader).
func ArtifactKey(group, actionName, etag string) string {
	return fmt.Sprintf("%s/%s/%s.js", sanitizeGroup(group), sanitize(actionName), sanitize(etag))
}

// SourceKey is the S3 object key suffix for TypeScript source.
func SourceKey(group, actionName, etag string) string {
	return fmt.Sprintf("%s/%s/%s.ts", sanitizeGroup(group), sanitize(actionName), sanitize(etag))
}

func sanitizeGroup(group string) string {
	g := sanitize(group)
	if g == "" {
		return "_"
	}
	return g
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "..", "_")
	return s
}
