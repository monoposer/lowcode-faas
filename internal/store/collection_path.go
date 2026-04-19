package store

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	maxCollectionPathLen  = 256
	maxCollectionSegments = 32
)

var collectionSegmentRe = regexp.MustCompile(`^[a-z0-9_-]+$`)

// NormalizeCollectionPath trims slashes, lowercases segments, validates.
// Empty string is the root collection (legacy /functions without prefix).
func NormalizeCollectionPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	raw = strings.Trim(raw, "/")
	if raw == "" {
		return "", nil
	}
	if len(raw) > maxCollectionPathLen {
		return "", fmt.Errorf("collection path too long")
	}
	parts := strings.Split(raw, "/")
	if len(parts) > maxCollectionSegments {
		return "", fmt.Errorf("collection path has too many segments")
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" || p == "." || p == ".." {
			return "", fmt.Errorf("invalid collection path segment")
		}
		pl := strings.ToLower(p)
		if !collectionSegmentRe.MatchString(pl) {
			return "", fmt.Errorf("invalid collection path segment %q", p)
		}
		out = append(out, pl)
	}
	return strings.Join(out, "/"), nil
}

// EffectiveCollectionPath returns normalized path; empty means root.
func EffectiveCollectionPath(raw string) (string, error) {
	return NormalizeCollectionPath(raw)
}

func collectionSegments(path string) []string {
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

// collectionDirOnDisk maps logical path "org/team" -> data/collections/org/team
func collectionDirOnDisk(path string) string {
	if path == "" {
		return filepath.Join(dataDir, "collections", "__root__")
	}
	return filepath.Join(append([]string{collectionsRoot()}, filepath.FromSlash(path))...)
}

func collectionFunctionsDirOnDisk(path string) string {
	return filepath.Join(collectionDirOnDisk(path), "functions")
}

// AncestorCollectionPaths 从根到叶子的路径链（含根 ""），用于按层级合并集合 env。
func AncestorCollectionPaths(raw string) ([]string, error) {
	path, err := NormalizeCollectionPath(raw)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return []string{""}, nil
	}
	segs := strings.Split(path, "/")
	out := []string{""}
	for i := range segs {
		out = append(out, strings.Join(segs[:i+1], "/"))
	}
	return out, nil
}
