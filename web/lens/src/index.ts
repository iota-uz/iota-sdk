// The public stylesheet is a first-class package artifact. Vite's library
// build only emits style.css for CSS reachable from the entry graph; consumers
// still opt into it explicitly through `@iota-uz/lens-web/styles.css`.
import './styles.css'

export { LensDashboard, type LensDashboardProps } from './LensDashboard'
export { LensDashboardElement, registerLensDashboardElement } from './element'
export {
  CONTRACT_VERSION,
  ContractVersionMismatchError,
  DashboardDocumentSchema,
  parseDocument,
  type DashboardDocument,
} from './contract'
export type { LensThemeMode } from './runtime'
