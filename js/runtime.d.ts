/**
 * Types for lowcode-faas action handlers (editor + compile-time).
 *
 * Invoke is modeled like HTTP: handler receives a Request, returns a Response.
 *
 * - Import types: `import type { ActionRequest, ActionResponse, HostAPI } from 'lowcode-faas/runtime'`
 * - `host` is injected by the worker qjs runtime as a **global** (not a real JS module) —
 *   use it directly; do not `import host`. See `internal/runner/host.go`.
 */
export type HostAPI = {
  log: (...args: unknown[]) => void
  nowMs: () => number
  echo: <T>(v: T) => T
  upper: (s: string) => string
  /** Opaque Go *MemoryKV (ProxyValue) — do not serialize */
  mem: unknown
  memSet: (mem: unknown, key: string, value: string) => void
  memGet: (mem: unknown, key: string) => string
  memLen: (mem: unknown) => number
  goCtx: () => unknown
  goCtxDeadlineMs: (ctx: unknown) => number
}

/**
 * Logical HTTP request passed to the action handler (single argument).
 *
 * Built from `POST /api/actions/{name}/invoke` JSON body fields
 * (`context`, `body`, `data`, `query`, `method`, `headers`, `path`)
 * or the legacy wrapper `{ input: ActionRequest }`.
 */
export type ActionRequest = {
  /** Caller / auth / tenant / invoke context */
  context?: Record<string, unknown>
  /** Parsed request body (JSON object, array, or scalar) */
  body?: unknown
  /** Path / route binding data (e.g. `{ id: "42" }`) */
  data?: Record<string, unknown>
  /** Query string parameters */
  query?: Record<string, string>
  /** HTTP method (default POST for invoke API) */
  method?: string
  /** Incoming headers (lower-cased keys recommended) */
  headers?: Record<string, string>
  /** Request path (optional; for logging / routing helpers) */
  path?: string
}

/**
 * Logical HTTP response returned by the action handler — also the invoke HTTP body.
 *
 * ```ts
 * { status: 200, data: { ... } }
 * ```
 *
 * No `logs` field: script logs go to worker slog only.
 * Bare return values are wrapped as `{ status: 200, data: value }`.
 * Legacy `{ body }` is mapped to `data`.
 */
export type ActionResponse = {
  status?: number
  data?: unknown
}

export type ActionHandler = (
  req: ActionRequest,
) => ActionResponse | Promise<ActionResponse>

/** @deprecated Use ActionRequest */
export type ActionInput = ActionRequest

declare global {
  /** Worker-injected host bindings (qjs). Available without import. */
  const host: HostAPI
}

export {}
