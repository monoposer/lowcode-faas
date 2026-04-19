package envutil

import (
	"fmt"
	"strings"
	"unicode"

	"lowcode-faas/internal/model"
)

const maxEnvKeyLen = 256
const maxEnvValLen = 4096

func ValidateEnvMap(m map[string]string) error {
	if m == nil {
		return nil
	}
	for k, v := range m {
		k = strings.TrimSpace(k)
		if k == "" {
			return fmt.Errorf("env key cannot be empty")
		}
		if len(k) > maxEnvKeyLen || len(v) > maxEnvValLen {
			return fmt.Errorf("env key or value too long")
		}
		if strings.ContainsAny(v, "\x00\n\r") {
			return fmt.Errorf("env values cannot contain null or newline characters")
		}
		if strings.EqualFold(k, "PORT") {
			return fmt.Errorf("env key PORT is reserved")
		}
		if !isValidEnvKey(k) {
			return fmt.Errorf("invalid env key: %q", k)
		}
	}
	return nil
}

func isValidEnvKey(k string) bool {
	for i, r := range k {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func MergeEnv(base, overlay map[string]string) map[string]string {
	out := make(map[string]string)
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}

func CloneStringMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func CloneFunction(fn *model.Function) *model.Function {
	if fn == nil {
		return nil
	}
	out := *fn
	out.CollectionPath = fn.CollectionPath
	out.Version = fn.Version
	out.DeploymentID = fn.DeploymentID
	out.Env = CloneStringMap(fn.Env)
	out.SourceCode = fn.SourceCode
	return &out
}
