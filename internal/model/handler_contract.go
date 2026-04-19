package model

// 用户函数须为 Node ESM，且导出：
//
//	export async function handler(input) { return { ok: true, data: ... }; }
//
// input: 与 invoke 请求体中的 input 字段相同（任意 JSON 对象）。
// 返回值须为 JSON 对象且包含布尔字段 ok：
//   - 成功：{ "ok": true, "data": <任意> }
//   - 业务失败：{ "ok": false, "error": "说明文字", "code": "可选机器码" }
//
// 平台将完整返回值写入 RunResult.output。
