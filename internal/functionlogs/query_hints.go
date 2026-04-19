package functionlogs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"lowcode-faas/internal/config"
)

// QueryHintsResponse GET /function-logs/query 返回体：各后端查询片段 + 可选 Grafana 链接。
// 各平台查询语言不同，无法做到「一条语句处处可执行」；此处统一 **字段语义**（run_id、function_name 等），由各查询串在对应 UI 中粘贴使用。
type QueryHintsResponse struct {
	RunID               string            `json:"run_id,omitempty"`
	FunctionName        string            `json:"function_name,omitempty"`
	CollectionPath      string            `json:"collection_path,omitempty"`
	DeploymentID        string            `json:"deployment_id,omitempty"`
	Queries             map[string]string `json:"queries"`
	GrafanaExploreURL   string            `json:"grafana_explore_url,omitempty"`
	Notes               string            `json:"notes,omitempty"`
}

// BuildExecutionLogQueries 生成与存储后端无关的「提示查询」；Loki 选择器需与 Promtail relabel 对齐后微调。
func BuildExecutionLogQueries(runID, functionName, collectionPath, deploymentID string) QueryHintsResponse {
	runID = strings.TrimSpace(runID)
	fn := strings.TrimSpace(functionName)
	cp := strings.TrimSpace(collectionPath)
	dep := strings.TrimSpace(deploymentID)

	// 子串匹配：不依赖 json 解析阶段，在任意把整行打进 message 的系统里可用
	sub := runID
	if sub == "" {
		sub = dep
	}
	if sub == "" {
		sub = fn
	}
	esc := func(s string) string { return strings.ReplaceAll(s, `"`, `\"`) }

	// Loki：{job="docker"} 为 Promtail docker_sd 常见标签；请与 deploy/promtail-dev.yml 对齐后修改
	lokiJSON := fmt.Sprintf(`{job="docker"} | json | run_id=%q`, runID)
	if runID == "" {
		lokiJSON = fmt.Sprintf(`{job="docker"} |= %q`, sub)
	}
	lokiLoose := fmt.Sprintf(`{job="docker"} |= %q`, sub)

	// Elasticsearch / OpenSearch Query String
	esQS := ""
	if runID != "" {
		esQS = fmt.Sprintf(`run_id:"%s"`, esc(runID))
	} else if fn != "" {
		esQS = fmt.Sprintf(`function_name:"%s"`, esc(fn))
	}
	esLucene := esQS

	// CloudWatch Logs Insights（JSON 在 @message 内）
	cwli := ""
	if runID != "" {
		cwli = fmt.Sprintf(`fields @timestamp, @message | filter @message like /%s/`, regexp.QuoteMeta(runID))
	}

	// Google Cloud Logging
	gcp := ""
	if runID != "" {
		gcp = fmt.Sprintf(`jsonPayload.run_id="%s" OR textPayload:"%s"`, esc(runID), esc(runID))
	}

	q := map[string]string{
		"loki_logql_json":   lokiJSON,
		"loki_logql_substr": lokiLoose,
	}
	if esQS != "" {
		q["elasticsearch_query_string"] = esQS
		q["elasticsearch_lucene"] = esLucene
	}
	if cwli != "" {
		q["aws_cloudwatch_logs_insights"] = cwli
	}
	if gcp != "" {
		q["google_cloud_logging"] = gcp
	}

	// 若配置了 Grafana Explore 模板（含 {{logql}} 占位，URL 编码由调用方在模板中自行处理或此处替换）
	out := QueryHintsResponse{
		RunID:           runID,
		FunctionName:    fn,
		CollectionPath:  cp,
		DeploymentID:    dep,
		Queries:         q,
		Notes:           "Loki 示例中的 {job=\"docker\"} 需与 Promtail 实际标签一致；生产环境请改为贵方 scrape 产生的 labels（如 compose_service）。",
	}
	if tpl := strings.TrimSpace(config.LogsGrafanaExploreURLTemplate); tpl != "" && (runID != "" || dep != "") {
		enc := url.QueryEscape(lokiLoose)
		out.GrafanaExploreURL = strings.ReplaceAll(tpl, "{{logql}}", enc)
		out.GrafanaExploreURL = strings.ReplaceAll(out.GrafanaExploreURL, "{{run_id}}", url.QueryEscape(runID))
		out.GrafanaExploreURL = strings.ReplaceAll(out.GrafanaExploreURL, "{{deployment_id}}", url.QueryEscape(dep))
	}
	return out
}

// ServeHTTPQueryHints GET /function-logs/query?run_id=&function=&collection_path=&deployment_id=
func ServeHTTPQueryHints(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	runID := q.Get("run_id")
	fn := q.Get("function")
	cp := q.Get("collection_path")
	dep := q.Get("deployment_id")
	if strings.TrimSpace(runID) == "" && strings.TrimSpace(fn) == "" && strings.TrimSpace(dep) == "" {
		http.Error(w, "run_id, function, or deployment_id is required", http.StatusBadRequest)
		return
	}
	h := BuildExecutionLogQueries(runID, fn, cp, dep)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h)
}
