import { createContext, useContext, type ReactNode } from 'react'
import type { NodeKey } from '../contract'
import type { ChartAnchor } from '../charts/adapter'

/**
 * Chrome a host (today: the explore panel) injects into the panel header, so
 * a drill trail and an explore affordance live in the card's own header
 * instead of adding rows around the card.
 */
export interface PanelChrome {
  trail?: ReactNode
  explore?: ReactNode
  /**
   * The panel's name is already on screen, so its own header must not repeat
   * it. Set by a tab group whose active tab label names the single panel
   * inside it: two borders, two radii, two header bars and «ЕЖЕДНЕВНЫЙ ДОХОД»
   * over «ДОХОД» forty pixels apart, all saying one thing. The card keeps its
   * header — export, expand and the ⓘ live there — and loses the duplicate
   * line; the accessible name comes from the section's own aria-label.
   */
  titleIsRedundant?: boolean
}

export const PanelChromeContext = createContext<PanelChrome | undefined>(undefined)

export function usePanelChrome(): PanelChrome | undefined {
  return useContext(PanelChromeContext)
}

/**
 * When a host handles mark activation itself, a chart reports the mark and
 * where it was clicked instead of drilling. Without a host the chart keeps
 * its direct drill behavior.
 */
export type MarkSelectionHandler = (key: NodeKey, anchor?: ChartAnchor) => void

export const MarkSelectionContext = createContext<MarkSelectionHandler | undefined>(undefined)

export function useMarkSelection(): MarkSelectionHandler | undefined {
  return useContext(MarkSelectionContext)
}
