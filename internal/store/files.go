package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"lowcode-faas/internal/envutil"
	"lowcode-faas/internal/model"
)

const dataDir = "data"

func legacyFunctionsDir() string {
	return filepath.Join(dataDir, "functions")
}

func ensureDataDir() error {
	if err := os.MkdirAll(collectionFunctionsDirOnDisk(""), 0o755); err != nil {
		return fmt.Errorf("create root collection functions dir: %w", err)
	}
	if err := os.MkdirAll(legacyFunctionsDir(), 0o755); err != nil {
		return fmt.Errorf("create legacy functions dir: %w", err)
	}
	return nil
}

func extForLanguage(lang string) (string, error) {
	if lang == model.LangJavaScript {
		return ".mjs", nil
	}
	return "", fmt.Errorf("unsupported language: %s", lang)
}

func functionSourcePathFiles(collectionPath, name, ext string) string {
	return filepath.Join(collectionFunctionsDirOnDisk(collectionPath), name+ext)
}

func functionEnvPathFiles(collectionPath, name string) string {
	return filepath.Join(collectionFunctionsDirOnDisk(collectionPath), name+".env.json")
}

func functionMetaPathFiles(collectionPath, name string) string {
	return filepath.Join(collectionFunctionsDirOnDisk(collectionPath), name+".meta.json")
}

func createFunctionFiles(ctx context.Context, collectionPath, name, language, source string, env map[string]string) (*model.Function, error) {
	collectionPath, err := NormalizeCollectionPath(collectionPath)
	if err != nil {
		return nil, err
	}
	ok, err := CollectionPathExists(ctx, collectionPath)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrCollectionNotFound
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("function name is required")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return nil, fmt.Errorf("invalid function name")
	}
	ext, err := extForLanguage(language)
	if err != nil {
		return nil, err
	}
	prevVer := readFunctionVersionFiles(collectionPath, name)
	nextVer := prevVer + 1
	if prevVer == 0 {
		nextVer = 1
	}
	now := time.Now().UTC()
	fn := &model.Function{
		ID:             functionQualifiedID(collectionPath, name),
		CollectionPath: collectionPath,
		Name:           name,
		Language:       language,
		SourceCode:     source,
		Env:            envutil.CloneStringMap(env),
		CreatedAt:      now,
		Version:        nextVer,
		DeploymentID:   model.FormatDeploymentID(collectionPath, name, nextVer),
	}
	path := functionSourcePathFiles(collectionPath, name, ext)
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		return nil, fmt.Errorf("write function file: %w", err)
	}
	if err := writeFunctionMetaVersion(collectionPath, name, nextVer); err != nil {
		return nil, fmt.Errorf("write function meta: %w", err)
	}
	envPath := functionEnvPathFiles(collectionPath, name)
	if len(env) == 0 {
		_ = os.Remove(envPath)
	} else {
		b, err := json.Marshal(env)
		if err != nil {
			return nil, fmt.Errorf("marshal env: %w", err)
		}
		if err := os.WriteFile(envPath, b, 0o600); err != nil {
			return nil, fmt.Errorf("write env file: %w", err)
		}
	}
	return fn, nil
}

type functionMetaJSON struct {
	Version int `json:"version"`
}

func readFunctionVersionFiles(collectionPath, name string) int {
	b, err := os.ReadFile(functionMetaPathFiles(collectionPath, name))
	if err != nil {
		return 0
	}
	var m functionMetaJSON
	if json.Unmarshal(b, &m) != nil {
		return 0
	}
	return m.Version
}

func writeFunctionMetaVersion(collectionPath, name string, v int) error {
	b, err := json.Marshal(functionMetaJSON{Version: v})
	if err != nil {
		return err
	}
	return os.WriteFile(functionMetaPathFiles(collectionPath, name), b, 0o644)
}

func listFunctionsFiles(ctx context.Context, collectionPath string) ([]*model.FunctionListItem, error) {
	collectionPath, err := NormalizeCollectionPath(collectionPath)
	if err != nil {
		return nil, err
	}
	ok, err := CollectionPathExists(ctx, collectionPath)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrCollectionNotFound
	}
	seen := make(map[string]struct{})
	out := make([]*model.FunctionListItem, 0, 16)

	addFromDir := func(dir string) error {
		matches, err := filepath.Glob(filepath.Join(dir, "*.mjs"))
		if err != nil {
			return err
		}
		for _, path := range matches {
			base := filepath.Base(path)
			if !strings.HasSuffix(base, ".mjs") {
				continue
			}
			nm := strings.TrimSuffix(base, ".mjs")
			if nm == "" {
				continue
			}
			if _, dup := seen[nm]; dup {
				continue
			}
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			ver := readFunctionVersionFiles(collectionPath, nm)
			if ver < 1 {
				ver = 1
			}
			seen[nm] = struct{}{}
			out = append(out, &model.FunctionListItem{
				ID:             functionQualifiedID(collectionPath, nm),
				CollectionPath: collectionPath,
				Name:           nm,
				Language:       model.LangJavaScript,
				Version:        ver,
				CreatedAt:      info.ModTime().UTC(),
			})
		}
		return nil
	}

	if err := addFromDir(collectionFunctionsDirOnDisk(collectionPath)); err != nil {
		return nil, err
	}
	if collectionPath == "" {
		_ = addFromDir(legacyFunctionsDir())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func resolveFunctionSourcePathFiles(collectionPath, name string) (path string, legacy bool, err error) {
	ext := ".mjs"
	p := functionSourcePathFiles(collectionPath, name, ext)
	if _, err := os.Stat(p); err == nil {
		return p, false, nil
	}
	if collectionPath == "" {
		leg := filepath.Join(legacyFunctionsDir(), name+ext)
		if _, err := os.Stat(leg); err == nil {
			return leg, true, nil
		}
	}
	return "", false, ErrNotFound
}

func getFunctionByNameFiles(ctx context.Context, collectionPath, name string, version *int) (*model.Function, error) {
	collectionPath, err := NormalizeCollectionPath(collectionPath)
	if err != nil {
		return nil, err
	}
	ok, err := CollectionPathExists(ctx, collectionPath)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrCollectionNotFound
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrNotFound
	}
	srcPath, legacy, err := resolveFunctionSourcePathFiles(collectionPath, name)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(srcPath)
	if err != nil {
		return nil, err
	}
	code, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, err
	}
	ver := readFunctionVersionFiles(collectionPath, name)
	if ver < 1 {
		ver = 1
	}
	if version != nil && *version != ver {
		return nil, ErrUnsupportedVersionPin
	}
	fn := &model.Function{
		ID:             functionQualifiedID(collectionPath, name),
		CollectionPath: collectionPath,
		Name:           name,
		Language:       model.LangJavaScript,
		SourceCode:     string(code),
		CreatedAt:      info.ModTime(),
		Version:        ver,
		DeploymentID:   model.FormatDeploymentID(collectionPath, name, ver),
	}
	envPath := functionEnvPathFiles(collectionPath, name)
	if legacy {
		envPath = filepath.Join(legacyFunctionsDir(), name+".env.json")
	}
	if b, err := os.ReadFile(envPath); err == nil {
		var env map[string]string
		if json.Unmarshal(b, &env) == nil && len(env) > 0 {
			fn.Env = env
		}
	}
	return fn, nil
}

func listFunctionVersionsFiles(ctx context.Context, collectionPath, name string) ([]model.FunctionVersionItem, error) {
	collectionPath, err := NormalizeCollectionPath(collectionPath)
	if err != nil {
		return nil, err
	}
	fn, err := getFunctionByNameFiles(ctx, collectionPath, name, nil)
	if err != nil {
		return nil, err
	}
	return []model.FunctionVersionItem{{Version: fn.Version, CreatedAt: fn.CreatedAt}}, nil
}

func functionFileMTimes(collectionPath, name, language string) (srcModNano, envModNano, metaModNano, collEnvModNano int64) {
	collectionPath, _ = NormalizeCollectionPath(collectionPath)
	if _, err := extForLanguage(language); err != nil {
		return 0, 0, 0, 0
	}
	if p, _, err := resolveFunctionSourcePathFiles(collectionPath, name); err == nil {
		if fi, err := os.Stat(p); err == nil {
			srcModNano = fi.ModTime().UnixNano()
		}
	}
	envP := functionEnvPathFiles(collectionPath, name)
	if fi, err := os.Stat(envP); err == nil {
		envModNano = fi.ModTime().UnixNano()
	} else if collectionPath == "" {
		leg := filepath.Join(legacyFunctionsDir(), name+".env.json")
		if fi, err := os.Stat(leg); err == nil {
			envModNano = fi.ModTime().UnixNano()
		}
	}
	if fi, err := os.Stat(functionMetaPathFiles(collectionPath, name)); err == nil {
		metaModNano = fi.ModTime().UnixNano()
	}
	chain, _ := AncestorCollectionPaths(collectionPath)
	for _, p := range chain {
		cp := filepath.Join(collectionDirOnDisk(p), "collection.env.json")
		if fi, err := os.Stat(cp); err == nil {
			n := fi.ModTime().UnixNano()
			if n > collEnvModNano {
				collEnvModNano = n
			}
		}
	}
	return srcModNano, envModNano, metaModNano, collEnvModNano
}
