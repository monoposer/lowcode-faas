package runner

import (
	"context"
	"io"
	"log"
	"os/exec"
	"strings"
	"time"

	"lowcode-faas/internal/config"
)

// StartBackgroundImagePrewarm 进程启动后后台 docker pull 用户运行时镜像，减轻首次 invoke 等待。
// runsDockerLocally：仅当本进程会执行 RunJS（worker 恒为 true；dispatcher 仅在未配置 worker URL 时为 true）。
func StartBackgroundImagePrewarm(runsDockerLocally bool) {
	if !runsDockerLocally || !config.PrewarmNodeImage {
		return
	}
	img := strings.TrimSpace(config.NodeDockerImage)
	if img == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
		defer cancel()
		log.Printf("prewarm: docker pull %s (background)", img)
		cmd := exec.CommandContext(ctx, "docker", "pull", img)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if err := cmd.Run(); err != nil {
			log.Printf("prewarm: pull %s failed: %v", img, err)
			return
		}
		log.Printf("prewarm: image ready %s", img)
	}()
}
