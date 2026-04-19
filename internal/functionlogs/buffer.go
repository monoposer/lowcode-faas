package functionlogs

import (
	"strings"
	"sync"
	"time"
)

const defaultMaxEntries = 10000

var (
	mu        sync.RWMutex
	entries   []Entry
	maxKeep   = defaultMaxEntries
	startedAt = time.Now()
)

// SetMaxEntries 设置内存环缓冲上限（用于搜索 API）。应在进程启动早期调用。
func SetMaxEntries(n int) {
	if n <= 0 {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	maxKeep = n
	if len(entries) > maxKeep {
		entries = entries[len(entries)-maxKeep:]
	}
}

// Append 追加一条日志（线程安全）。
func Append(e Entry) {
	mu.Lock()
	defer mu.Unlock()
	if maxKeep <= 0 {
		maxKeep = defaultMaxEntries
	}
	entries = append(entries, e)
	if len(entries) > maxKeep {
		entries = entries[len(entries)-maxKeep:]
	}
}

// Query 在内存缓冲中筛选（子串匹配，大小写不敏感）。
func Query(runID, function, collectionPath, q string, limit int) []Entry {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	runID = strings.TrimSpace(runID)
	function = strings.TrimSpace(function)
	collectionPath = strings.TrimSpace(collectionPath)
	q = strings.TrimSpace(strings.ToLower(q))

	mu.RLock()
	defer mu.RUnlock()

	out := make([]Entry, 0, limit)
	for i := len(entries) - 1; i >= 0 && len(out) < limit; i-- {
		e := entries[i]
		if runID != "" && !strings.EqualFold(e.RunID, runID) {
			continue
		}
		if function != "" && !strings.EqualFold(e.FunctionName, function) {
			continue
		}
		if collectionPath != "" && !strings.EqualFold(e.CollectionPath, collectionPath) {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(e.Message), q) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// Stats 用于调试（可选）。
func Stats() (n int, since time.Time) {
	mu.RLock()
	defer mu.RUnlock()
	return len(entries), startedAt
}
