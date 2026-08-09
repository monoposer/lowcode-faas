package tscompile

import (
	"fmt"
	"strings"
)

// ArtifactKey is the S3 object key suffix for compiled JS (prefixed with action-js/ by the uploader).
func ArtifactKey(actionName, etag string) string {
	return fmt.Sprintf("%s/%s.js", sanitize(actionName), sanitize(etag))
}

// SourceKey is the S3 object key suffix for TypeScript source.
func SourceKey(actionName, etag string) string {
	return fmt.Sprintf("%s/%s.ts", sanitize(actionName), sanitize(etag))
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "..", "_")
	return s
}
