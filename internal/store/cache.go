package store

import (
	"context"
	"strings"
	"sync"

	"lowcode-faas/internal/config"
	"lowcode-faas/internal/envutil"
	"lowcode-faas/internal/model"
)

type cacheEntry struct {
	fn             *model.Function
	dbVersion      int
	srcModNano     int64
	envModNano     int64
	metaModNano    int64
	collEnvModNano int64
}

var (
	cacheMu sync.RWMutex
	cache   = make(map[string]*cacheEntry)
)

func cacheKey(collectionPath, name string) string {
	cp, _ := NormalizeCollectionPath(collectionPath)
	return cp + "\x00" + strings.TrimSpace(name)
}

func cacheGet(ctx context.Context, collectionPath, name string) (*model.Function, error) {
	key := cacheKey(collectionPath, name)
	if strings.TrimSpace(name) == "" {
		return nil, ErrNotFound
	}
	cacheMu.RLock()
	e := cache[key]
	if e != nil && cacheEntryStillGood(ctx, collectionPath, name, e) {
		out := envutil.CloneFunction(e.fn)
		cacheMu.RUnlock()
		return out, nil
	}
	cacheMu.RUnlock()

	cacheMu.Lock()
	defer cacheMu.Unlock()
	e = cache[key]
	if e != nil && cacheEntryStillGood(ctx, collectionPath, name, e) {
		return envutil.CloneFunction(e.fn), nil
	}
	fn, err := getFunctionByNameUncached(ctx, collectionPath, name, nil)
	if err != nil {
		return nil, err
	}
	stored := envutil.CloneFunction(fn)
	ent := &cacheEntry{fn: stored}
	if storageDBS3() {
		ent.dbVersion = fn.Version
	} else {
		sm, em, mm, cm := functionFileMTimes(collectionPath, name, fn.Language)
		ent.srcModNano, ent.envModNano, ent.metaModNano, ent.collEnvModNano = sm, em, mm, cm
	}
	cache[key] = ent
	return envutil.CloneFunction(stored), nil
}

func cachePutAfterMutation(fn *model.Function) {
	if !config.FunctionCacheEnabled || fn == nil {
		return
	}
	key := cacheKey(fn.CollectionPath, fn.Name)
	if strings.TrimSpace(fn.Name) == "" {
		return
	}
	stored := envutil.CloneFunction(fn)
	ent := &cacheEntry{fn: stored}
	if storageDBS3() {
		ent.dbVersion = fn.Version
	} else {
		sm, em, mm, cm := functionFileMTimes(fn.CollectionPath, fn.Name, fn.Language)
		ent.srcModNano, ent.envModNano, ent.metaModNano, ent.collEnvModNano = sm, em, mm, cm
	}
	cacheMu.Lock()
	cache[key] = ent
	cacheMu.Unlock()
}

func cacheEntryStillGood(ctx context.Context, collectionPath, name string, e *cacheEntry) bool {
	if storageDBS3() {
		if appDB == nil {
			return false
		}
		cp, err := NormalizeCollectionPath(collectionPath)
		if err != nil {
			return false
		}
		var cur int
		err = appDB.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(version), 0) FROM function_versions WHERE collection_path = $1 AND function_name = $2`,
			cp, name).Scan(&cur)
		if err != nil {
			return false
		}
		return cur == e.dbVersion
	}
	sm, em, mm, cm := functionFileMTimes(collectionPath, name, e.fn.Language)
	return sm == e.srcModNano && em == e.envModNano && mm == e.metaModNano && cm == e.collEnvModNano
}
