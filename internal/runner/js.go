package runner

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"lowcode-faas/internal/config"
	"lowcode-faas/internal/envutil"
	"lowcode-faas/internal/functionlogs"
	"lowcode-faas/internal/limits"
	"lowcode-faas/internal/model"
)

//go:embed bootstrap.mjs
var bootstrapSource string

// RunJS 在 Docker 内执行用户 handler（无 HTTP 端口；读写 input.json / output.json）
// fn.Env 在调用方已包含「集合祖先链 ∪ 函数」合并结果；此处再与 invoke 环境合并。
func RunJS(ctx context.Context, fn *model.Function, input json.RawMessage, invokeEnv map[string]string, timeout time.Duration) (*model.RunResult, error) {
	runID := model.NewRunID("run")
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := limits.AcquireRun(ctx); err != nil {
		return failedResult(runID, fn, start, err.Error()), nil
	}
	defer limits.ReleaseRun()

	tmpDir, err := os.MkdirTemp("", "lowcode-faas-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() {
		functionlogs.ProcessFileAfterRun(context.Background(), runID, fn, tmpDir)
		_ = os.RemoveAll(tmpDir)
	}()

	if err := writeWorkspace(tmpDir, fn.SourceCode, input); err != nil {
		return nil, err
	}
	merged := envutil.MergeEnv(fn.Env, invokeEnv)
	merged = envutil.StripLowcodeInternalEnvKeys(merged)
	internal := map[string]string{}
	if config.FunctionLogsEnabled {
		internal["LOWCODE_FAAS_RUN_ID"] = runID
		internal["LOWCODE_FAAS_EXECUTION_ID"] = runID
		internal["LOWCODE_FAAS_FUNCTION_NAME"] = fn.Name
		internal["LOWCODE_FAAS_COLLECTION_PATH"] = fn.CollectionPath
		internal["LOWCODE_FAAS_DEPLOYMENT_VERSION"] = strconv.Itoa(fn.Version)
		internal["LOWCODE_FAAS_DEPLOYMENT_ID"] = fn.DeploymentID
		if internal["LOWCODE_FAAS_DEPLOYMENT_ID"] == "" {
			internal["LOWCODE_FAAS_DEPLOYMENT_ID"] = model.FormatDeploymentID(fn.CollectionPath, fn.Name, fn.Version)
		}
		internal["LOWCODE_FAAS_LOG_CAPTURE"] = "1"
	}

	var stderrBuf strings.Builder
	createArgs := dockerCreateArgs(merged, internal, config.NodeDockerImage, "node", "bootstrap.mjs")
	cmdCreate := exec.CommandContext(ctx, "docker", createArgs...)
	cmdCreate.Stderr = &stderrBuf
	out, err := cmdCreate.Output()
	if err != nil {
		return failedResult(runID, fn, start, fmt.Sprintf("docker create: %v; stderr: %s", err, strings.TrimSpace(stderrBuf.String()))), nil
	}
	cid := strings.TrimSpace(string(out))
	if cid == "" {
		return failedResult(runID, fn, start, "docker create: empty container id"), nil
	}
	defer func() {
		_ = exec.CommandContext(context.Background(), "docker", "rm", "-f", cid).Run()
	}()

	srcWorkspace := filepath.Clean(tmpDir) + string(filepath.Separator) + "."
	cpIn := exec.CommandContext(ctx, "docker", "cp", srcWorkspace, cid+":/workspace/")
	cpIn.Stderr = &stderrBuf
	if err := cpIn.Run(); err != nil {
		return failedResult(runID, fn, start, fmt.Sprintf("docker cp in: %v; stderr: %s", err, strings.TrimSpace(stderrBuf.String()))), nil
	}

	cmdStart := exec.CommandContext(ctx, "docker", "start", "-a", cid)
	cmdStart.Stdout = os.Stdout
	cmdStart.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
	startErr := cmdStart.Run()
	copyOutputsFromContainer(ctx, cid, tmpDir, &stderrBuf)
	if startErr != nil {
		return failedResult(runID, fn, start, fmt.Sprintf("container: %v; stderr: %s", startErr, strings.TrimSpace(stderrBuf.String()))), nil
	}

	outPath := filepath.Join(tmpDir, "output.json")
	raw, err := os.ReadFile(outPath)
	if err != nil {
		return failedResult(runID, fn, start, "missing output.json after run"), nil
	}
	var probe map[string]interface{}
	if json.Unmarshal(raw, &probe) != nil {
		return failedResult(runID, fn, start, "output.json is not valid JSON"), nil
	}
	outJSON := json.RawMessage(raw)
	duration := time.Since(start)
	res := &model.RunResult{
		RunID:             runID,
		ExecutionID:       runID,
		Status:            "success",
		HTTPStatusCode:    200,
		HTTPHeaders:       map[string]string{"Content-Type": "application/json"},
		Output:            outJSON,
		DurationMs:        duration.Milliseconds(),
		FunctionID:        fn.ID,
		CollectionPath:    fn.CollectionPath,
		DeploymentVersion: fn.Version,
		DeploymentID:      fn.DeploymentID,
	}
	if res.DeploymentID == "" {
		res.DeploymentID = model.FormatDeploymentID(fn.CollectionPath, fn.Name, fn.Version)
	}
	return res, nil
}

func copyOutputsFromContainer(ctx context.Context, cid, tmpDir string, stderrBuf *strings.Builder) {
	outSrc := cid + ":/workspace/output.json"
	outDest := filepath.Join(tmpDir, "output.json")
	cmd := exec.CommandContext(ctx, "docker", "cp", outSrc, outDest)
	cmd.Stderr = stderrBuf
	_ = cmd.Run()

	logSrc := cid + ":/workspace/" + functionlogs.WorkspaceUserLogFile
	logDest := filepath.Join(tmpDir, functionlogs.WorkspaceUserLogFile)
	cmd2 := exec.CommandContext(ctx, "docker", "cp", logSrc, logDest)
	cmd2.Stderr = stderrBuf
	_ = cmd2.Run()
}

func failedResult(runID string, fn *model.Function, start time.Time, msg string) *model.RunResult {
	fnID := ""
	cp := ""
	ver := 0
	dep := ""
	if fn != nil {
		fnID = fn.ID
		cp = fn.CollectionPath
		ver = fn.Version
		dep = fn.DeploymentID
		if dep == "" && fn.Name != "" {
			dep = model.FormatDeploymentID(fn.CollectionPath, fn.Name, fn.Version)
		}
	}
	return &model.RunResult{
		RunID:             runID,
		ExecutionID:       runID,
		Status:            "failed",
		HTTPStatusCode:    0,
		Output:            nil,
		ErrorMessage:      msg,
		DurationMs:        time.Since(start).Milliseconds(),
		FunctionID:        fnID,
		CollectionPath:    cp,
		DeploymentVersion: ver,
		DeploymentID:      dep,
	}
}

func writeWorkspace(dir, userCode string, input json.RawMessage) error {
	if err := os.WriteFile(filepath.Join(dir, "bootstrap.mjs"), []byte(bootstrapSource), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "function.mjs"), []byte(userCode), 0o644); err != nil {
		return err
	}
	body := input
	if len(body) == 0 || string(body) == "null" {
		body = []byte("{}")
	}
	return os.WriteFile(filepath.Join(dir, "input.json"), body, 0o644)
}

func dockerCreateArgs(mergedEnv, internalEnv map[string]string, image string, containerCmd ...string) []string {
	args := []string{"create", "--pull=missing"}
	if s := strings.TrimSpace(config.DockerFunctionMemory); s != "" {
		args = append(args, "--memory="+s)
	}
	if s := strings.TrimSpace(config.DockerFunctionCPUs); s != "" {
		args = append(args, "--cpus="+s)
	}
	if s := strings.TrimSpace(config.DockerNetworkMode); s != "" {
		args = append(args, "--network="+s)
	}
	args = append(args, "-w", "/workspace")
	for k, v := range mergedEnv {
		if strings.EqualFold(k, "PORT") {
			continue
		}
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	for k, v := range internalEnv {
		if strings.TrimSpace(k) == "" {
			continue
		}
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	args = append(args, image)
	args = append(args, containerCmd...)
	return args
}
