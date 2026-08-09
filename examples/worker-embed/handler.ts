/**
 * Demo action for examples/worker-embed (custom host.greet).
 *
 * Base types: lowcode-faas/runtime (js/runtime.d.ts).
 * Custom host: merge examples/worker-embed/host.d.ts in your Monaco/editor.
 */
import type { ActionRequest, ActionResponse } from 'lowcode-faas/runtime'

export default function handler(req: ActionRequest): ActionResponse {
  const body = (req.body ?? {}) as Record<string, unknown>
  const name = String(body.name ?? 'faas')

  host.log('worker-embed demo', name)
  const greeting = host.greet(name)

  return {
    status: 200,
    data: {
      ok: true,
      greeting,
      upper: host.upper(name),
      at: host.nowMs(),
    },
  }
}
