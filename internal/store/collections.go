package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"lowcode-faas/internal/envutil"
	"lowcode-faas/internal/model"
)

var (
	ErrCollectionNotFound = errors.New("collection not found")
	ErrCollectionExists   = errors.New("collection already exists")
)

func collectionsRoot() string {
	return filepath.Join(dataDir, "collections")
}

// mkdirCollectionPath creates data/collections/<seg1>/.../<segN>/functions for each prefix.
func mkdirCollectionPath(path string) error {
	segs := collectionSegments(path)
	if len(segs) == 0 {
		return fmt.Errorf("collection path is required")
	}
	for i := range segs {
		partial := strings.Join(segs[:i+1], "/")
		d := collectionFunctionsDirOnDisk(partial)
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// CreateCollection 创建嵌套集合路径（mkdir -p）；db_s3 写入 collection_paths；可选写入集合 env
func CreateCollection(ctx context.Context, rawPath string, env map[string]string) (*model.CollectionListItem, error) {
	path, err := NormalizeCollectionPath(rawPath)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, fmt.Errorf("collection path is required")
	}
	if storageDBS3() {
		item, err := createCollectionPGS3(ctx, path)
		if err != nil {
			return nil, err
		}
		if len(env) > 0 {
			if err := setCollectionEnvPGS3(ctx, path, env); err != nil {
				return nil, err
			}
		}
		return item, nil
	}
	if err := ensureDataDir(); err != nil {
		return nil, err
	}
	leafDir := collectionDirOnDisk(path)
	if st, err := os.Stat(leafDir); err == nil && st.IsDir() {
		return nil, ErrCollectionExists
	}
	if err := mkdirCollectionPath(path); err != nil {
		return nil, err
	}
	info, err := os.Stat(leafDir)
	if err != nil {
		return nil, err
	}
	item := &model.CollectionListItem{Path: path, CreatedAt: info.ModTime().UTC()}
	if len(env) > 0 {
		if err := SetCollectionEnv(ctx, path, env); err != nil {
			return nil, err
		}
	}
	return item, nil
}

// ListCollections 列出已登记的集合路径
func ListCollections(ctx context.Context, prefix string) ([]*model.CollectionListItem, error) {
	prefix, err := NormalizeCollectionPath(prefix)
	if err != nil {
		return nil, err
	}
	if storageDBS3() {
		return listCollectionsPGS3(ctx, prefix)
	}
	return listCollectionsFromDisk(prefix)
}

func listCollectionsFromDisk(prefix string) ([]*model.CollectionListItem, error) {
	root := collectionsRoot()
	_ = os.MkdirAll(root, 0o755)
	var out []*model.CollectionListItem
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if d.Name() == "__root__" {
			return filepath.SkipDir
		}
		funcDir := filepath.Join(path, "functions")
		fi, err := os.Stat(funcDir)
		if err != nil || !fi.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if prefix != "" && rel != prefix && !strings.HasPrefix(rel, prefix+"/") {
			return nil
		}
		info, _ := d.Info()
		ct := time.Time{}
		if info != nil {
			ct = info.ModTime().UTC()
		}
		out = append(out, &model.CollectionListItem{Path: rel, CreatedAt: ct})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, err
}

// GetCollectionEnv 读取集合级环境变量
func GetCollectionEnv(ctx context.Context, collectionPath string) (map[string]string, error) {
	collectionPath, err := NormalizeCollectionPath(collectionPath)
	if err != nil {
		return nil, err
	}
	if storageDBS3() {
		return getCollectionEnvPGS3(ctx, collectionPath)
	}
	p := filepath.Join(collectionDirOnDisk(collectionPath), "collection.env.json")
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var m map[string]string
	if json.Unmarshal(b, &m) != nil || len(m) == 0 {
		return nil, nil
	}
	return m, nil
}

// SetCollectionEnv 写入集合级环境
func SetCollectionEnv(ctx context.Context, collectionPath string, env map[string]string) error {
	collectionPath, err := NormalizeCollectionPath(collectionPath)
	if err != nil {
		return err
	}
	if err := envutil.ValidateEnvMap(env); err != nil {
		return err
	}
	if storageDBS3() {
		return setCollectionEnvPGS3(ctx, collectionPath, env)
	}
	if err := ensureDataDir(); err != nil {
		return err
	}
	if _, err := os.Stat(collectionDirOnDisk(collectionPath)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrCollectionNotFound
		}
		return err
	}
	p := filepath.Join(collectionDirOnDisk(collectionPath), "collection.env.json")
	if len(env) == 0 {
		return os.Remove(p)
	}
	b, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

// MergedAncestorCollectionEnvs 合并从根到当前路径上每一层的 collection.env（子路径覆盖父路径同名键）。
func MergedAncestorCollectionEnvs(ctx context.Context, collectionPath string) (map[string]string, error) {
	chain, err := AncestorCollectionPaths(collectionPath)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string)
	for _, p := range chain {
		m, err := GetCollectionEnv(ctx, p)
		if err != nil {
			return nil, err
		}
		out = envutil.MergeEnv(out, m)
	}
	return out, nil
}

// CollectionPathExists 用于创建函数前校验
func CollectionPathExists(ctx context.Context, collectionPath string) (bool, error) {
	collectionPath, err := NormalizeCollectionPath(collectionPath)
	if err != nil {
		return false, err
	}
	if collectionPath == "" {
		return true, nil
	}
	if storageDBS3() {
		return collectionPathExistsPGS3(ctx, collectionPath)
	}
	st, err := os.Stat(collectionDirOnDisk(collectionPath))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return st.IsDir(), nil
}
