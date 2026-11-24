import { createContext, useContext, type Accessor } from 'solid-js'
import type { ToastController } from '@iota-uz/sdk/solid-ui'
import type { Props, RPC } from './Automations.solid.generated'
import type { Translator } from './model'

export type Call = <K extends keyof RPC>(method: K, params: RPC[K]['params']) => Promise<RPC[K]['result']>

/** What every part of the screens shares: strings, the RPC, the route's props and the clock. */
export interface Ctx {
  t: Translator
  lang: string
  call: Call
  props: Props
  toast: ToastController
  /** Now, moved once a minute (S6): relative times re-render together. */
  now: Accessor<Date>
  navigate(path: string): void
}

export const AppContext = createContext<Ctx>()

export function useApp(): Ctx {
  const ctx = useContext(AppContext)
  if (!ctx) throw new Error('automations: AppContext is missing')
  return ctx
}
