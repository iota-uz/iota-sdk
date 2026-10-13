import { createContext, useContext, type JSX } from 'solid-js'
import type { ContextFilters, ContextOrigin } from '../contract/types'

/** Persistence is opt-in and remains authorized/versioned by the host. */
export interface FilterDefaultsHost {
  canSave: (origin: ContextOrigin) => boolean
  saveDefaults: (origin: ContextOrigin, filters: ContextFilters) => Promise<void>
}

const FilterDefaultsContext = createContext<FilterDefaultsHost>()
export function FilterDefaultsProvider(props: { host?: FilterDefaultsHost; children: JSX.Element }) {
  return <FilterDefaultsContext.Provider value={props.host}>{props.children}</FilterDefaultsContext.Provider>
}
export function useFilterDefaultsHost() { return useContext(FilterDefaultsContext) }
