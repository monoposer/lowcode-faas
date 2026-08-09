/**
 * Example action: call Go host functions from JS (worker qjs + ProxyValue memory).
 *
 * Types: `import type { ActionRequest, ActionResponse } from 'lowcode-faas/runtime'`.
 * `host` is a worker global (do not import the value).
 */
import type { ActionRequest, ActionResponse } from 'lowcode-faas/runtime'

export default function handler(req: ActionRequest): ActionResponse {
  const body = (req.body ?? {}) as Record<string, unknown>
  const key = String(body.key ?? 'demo')
  const value = String(body.value ?? 'from-js')

  host.log('host-example', key, value, req.context)

  const upper = host.upper(key)
  const echoed = host.echo({ key, value, at: host.nowMs() })

  host.memSet(host.mem, key, value)
  const fromGoMem = host.memGet(host.mem, key)
  const size = host.memLen(host.mem)

  const gctx = host.goCtx()
  const deadlineMs = host.goCtxDeadlineMs(gctx)

  return {
    status: 200,
    data: {
      ok: true,
      upper,
      echoed,
      fromGoMem,
      size,
      deadlineMs,
      query: req.query ?? null,
      data: req.data ?? null,
    },
  }
}
