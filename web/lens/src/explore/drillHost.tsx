import { createContext, useContext, type JSX } from 'solid-js'
import { useOptionalDashboard } from '../runtime/provider'
import { breadcrumbsForNavigation } from './model'
import type { InvestigationHost } from './investigation'
import { navigationToURL } from '../runtime/url'
import { withReturnContext } from '../runtime/returnContext'

/**
 * What a host lets an exploration do beyond the board: save the slice as a
 * report (investigation) and carry the exploration into its own pages
 * (returnContext adds `lens-return` to links that leave Lens).
 */
export interface LensDrillHost {
  investigation?: InvestigationHost
  returnContext?: boolean
}

const DrillHostContext = createContext<LensDrillHost>({})

export function DrillHostProvider(props: { host?: LensDrillHost; children: JSX.Element }) {
  return <DrillHostContext.Provider value={props.host ?? {}}>{props.children}</DrillHostContext.Provider>
}

export function useDrillHost(): LensDrillHost {
  return useContext(DrillHostContext)
}

/**
 * Decorates a link that leaves Lens for a host page with the return context
 * of the current exploration. Read at render time: the exploration URL is
 * derived from the runtime's navigation, not from a location that may not be
 * synchronized yet. Without a host opt-in, or outside a runtime, links pass
 * through unchanged.
 */
export function useReturnLink(): (href: string | undefined) => string | undefined {
  const host = useDrillHost()
  const dashboard = useOptionalDashboard()
  return (href) => {
    if (!href || !host.returnContext || !dashboard || typeof globalThis.location === 'undefined') return href
    const document = dashboard.document
    const navigation = dashboard.navigation
    const location = new URL(globalThis.location.href)
    const current = navigation.drawer || !navigation.path.length ? location : navigationToURL(navigation, location)
    const panel = document.panels.find((candidate) => candidate.id === navigation.panelId)
    const trail = panel && navigation.path.length ? breadcrumbsForNavigation(document, panel, navigation).map((crumb) => crumb.label) : []
    const title = document.header?.title?.trim() || document.meta.title
    return withReturnContext(href, current, { title, trail })
  }
}
