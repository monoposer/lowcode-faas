package functionlogs

import "lowcode-faas/internal/config"

// InitFromConfig 根据配置初始化内存缓冲大小等。
func InitFromConfig() {
	SetMaxEntries(config.FunctionLogsBufferMax)
}
