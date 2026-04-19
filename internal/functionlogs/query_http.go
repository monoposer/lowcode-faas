package functionlogs

import (
	"encoding/json"
	"net/http"
	"strconv"

	"lowcode-faas/internal/config"
)

// ServeHTTPQuery GET 查询内存中的函数日志（run_id / function / q 子串过滤）。
func ServeHTTPQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !config.FunctionLogsEnabled {
		http.Error(w, "function logs are disabled", http.StatusNotFound)
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	rows := Query(q.Get("run_id"), q.Get("function"), q.Get("collection_path"), q.Get("q"), limit)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rows)
}
