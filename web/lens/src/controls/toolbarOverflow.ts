import { createContext, useContext, type Accessor } from 'solid-js'

/** Where the dashboard-scope controls put their secondary verbs in a compact bar; undefined keeps them inline. */
export const ToolbarOverflowContext = createContext<Accessor<HTMLElement | undefined>>()

export function useToolbarOverflow(): Accessor<HTMLElement | undefined> | undefined {
  return useContext(ToolbarOverflowContext)
}
