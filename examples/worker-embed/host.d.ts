/**
 * Demo: customer-owned HostAPI extensions for the editor.
 *
 * The worker SDK only ships base types in `js/runtime.d.ts`.
 * Your frontend should add this file (or your own) via Monaco extraLib / tsconfig paths.
 *
 * Keep in sync with Go binders registered via worker.WithHost in main.go.
 */
import type { HostAPI as BaseHostAPI } from 'lowcode-faas/runtime'

export type HostAPI = BaseHostAPI & {
  /** Demo custom host from examples/worker-embed */
  greet: (name: string) => string
}

declare global {
  const host: HostAPI
}

export {}
