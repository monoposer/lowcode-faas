/**
 * 用户函数须导出 `handler`，签名为：
 * `export async function handler(input: HandlerInput): Promise<HandlerOutput>`
 *
 * 将本文件路径加入 tsconfig 的 `compilerOptions.types` 或三斜线引用：
 * /// <reference path="../js/runtime.d.ts" />
 */

export type HandlerInput = Record<string, unknown>;

export type HandlerOutput =
  | { ok: true; data?: unknown }
  | { ok: false; error: string; code?: string };
