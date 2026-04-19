package envutil

import "strings"

// StripLowcodeInternalEnvKeys 移除用户传入的 LOWCODE_FAAS_*，避免覆盖平台注入的运行期变量。
func StripLowcodeInternalEnvKeys(m map[string]string) map[string]string {
	if len(m) == 0 {
		return m
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if strings.HasPrefix(strings.ToUpper(k), "LOWCODE_FAAS_") {
			continue
		}
		out[k] = v
	}
	return out
}
