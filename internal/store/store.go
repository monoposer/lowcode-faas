package store

import (
	"context"
	"errors"
	"strings"

	"lowcode-faas/internal/config"
	"lowcode-faas/internal/model"
)

var (
	ErrNotFound              = errors.New("not found")
	ErrUnsupportedLanguage   = errors.New(`only language "javascript" is supported`)
	ErrUnsupportedVersionPin = errors.New("version pin requires db_s3 storage or a matching current version in files mode")
)

// Init 初始化存储后端
func Init(ctx context.Context) error {
	if storageDBS3() {
		return initPGS3(ctx)
	}
	return ensureDataDir()
}

func storageDBS3() bool {
	return strings.EqualFold(strings.TrimSpace(config.StorageBackend), "db_s3")
}

// CreateFunction 创建或更新函数（仅持久化函数级 env；invoke 前由调用方合并集合祖先链 env）
func CreateFunction(ctx context.Context, collectionPath, name, language, source string, env map[string]string) (*model.Function, error) {
	if language == "" {
		language = model.LangJavaScript
	}
	if language != model.LangJavaScript {
		return nil, ErrUnsupportedLanguage
	}
	var fn *model.Function
	var err error
	if storageDBS3() {
		fn, err = createFunctionPGS3(ctx, collectionPath, name, language, source, env)
	} else {
		fn, err = createFunctionFiles(ctx, collectionPath, name, language, source, env)
	}
	if err != nil {
		return nil, err
	}
	if config.FunctionCacheEnabled {
		cachePutAfterMutation(fn)
	}
	return fn, nil
}

// GetFunctionByName 加载函数（仅函数级 env）。version 非空时加载指定部署版本（并跳过缓存）。
func GetFunctionByName(ctx context.Context, collectionPath, name string, version *int) (*model.Function, error) {
	if version != nil || !config.FunctionCacheEnabled {
		return getFunctionByNameUncached(ctx, collectionPath, name, version)
	}
	return cacheGet(ctx, collectionPath, name)
}

func getFunctionByNameUncached(ctx context.Context, collectionPath, name string, version *int) (*model.Function, error) {
	if storageDBS3() {
		return getFunctionByNamePGS3(ctx, collectionPath, name, version)
	}
	return getFunctionByNameFiles(ctx, collectionPath, name, version)
}

// ListFunctions 列出集合内函数（不含源码）
func ListFunctions(ctx context.Context, collectionPath string) ([]*model.FunctionListItem, error) {
	if storageDBS3() {
		return listFunctionsPGS3(ctx, collectionPath)
	}
	return listFunctionsFiles(ctx, collectionPath)
}

// ListFunctionVersions 列出部署版本（db_s3 多条；files 仅当前一条）
func ListFunctionVersions(ctx context.Context, collectionPath, name string) ([]model.FunctionVersionItem, error) {
	if storageDBS3() {
		return listFunctionVersionsPGS3(ctx, collectionPath, name)
	}
	return listFunctionVersionsFiles(ctx, collectionPath, name)
}
