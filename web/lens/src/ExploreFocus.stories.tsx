import type { Story } from '@ladle/react'
import { useState } from 'react'
import fixture from '../fixtures/focus.json'
import { parseDocument } from './contract'
import { DashboardPanels } from './DashboardPanels'
import { DashboardRuntimeProvider, DocumentProvider, navigationToURL } from './runtime'
import './styles.css'

/**
 * The focus-canvas rework of the metric exploration surface: the drill root
 * opts in through `presentation.focus = "canvas"`, and an actively exploring
 * panel wears the focus chrome — breadcrumb, metric context with the parent
 * mini-chart, lens selector — with the collapsed source-data disclosure below
 * the chart. The resting card and non-focus documents render exactly as before.
 */
const focusDocument = parseDocument(fixture)

/**
 * The production shape of the same exploration: the host shares its grid row
 * with a neighbor at span 6, and — exactly as the Go builder emits — every
 * perspective root level records only its own perspective ref, so the sibling
 * set the lens selector offers has to be derived from the branch, not read off
 * the level.
 */
const halfWidthRaw = JSON.parse(JSON.stringify(fixture)) as {
  panels: Array<Record<string, unknown>>
  layout: { rows: Array<{ heading?: string; panels: Array<Record<string, unknown>> }> }
  drill: { edges: Record<string, { perspectives: Array<{ id: string }> }> }
}
const halfWidthNeighbor: Record<string, unknown> = {
  ...halfWidthRaw.panels[0]!,
  id: 'premium-neighbor',
  title: 'Neighbor card',
  terminal: true,
  actions: [],
}
delete halfWidthNeighbor.drillRoot
halfWidthRaw.panels.push(halfWidthNeighbor)
halfWidthRaw.layout.rows[0]!.panels = [
  { panelId: 'premium', span: 6 },
  { panelId: 'premium-neighbor', span: 6 },
]
halfWidthRaw.drill.edges['premium/motor']!.perspectives = [{ id: 'premium/motor/by-product' }]
halfWidthRaw.drill.edges['premium/motor/trend']!.perspectives = [{ id: 'premium/motor/trend' }]
halfWidthRaw.drill.edges['premium/motor/evidence']!.perspectives = [{ id: 'premium/motor/evidence' }]
const halfWidthDocument = parseDocument(halfWidthRaw)

const drilledPath = ['premium', 'premium/motor']
const deepPath = ['premium', 'premium/motor', 'premium/motor/new', 'premium/motor/new/direct']

/**
 * Pins the navigation to a level of the focus fixture before the runtime
 * mounts, exactly as a shared deep link would arrive.
 */
function AtPath({ theme, path, perspectiveId, width, document = focusDocument }: {
  theme: 'light' | 'dark'
  path: Array<string>
  perspectiveId?: string
  width?: string
  document?: typeof focusDocument
}) {
  useState(() => {
    window.history.replaceState(null, '', navigationToURL(
      { panelId: document.panels[0]!.id, path, ...(perspectiveId ? { perspectiveId } : {}) },
      new URL(window.location.href),
    ))
    return true
  })
  return (
    <div className="lens-root" data-theme={theme} style={width ? { width } : undefined}>
      <DocumentProvider initialDocument={document}>
        <DashboardRuntimeProvider locale="en"><DashboardPanels /></DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

/**
 * The canvas standing at its drill root: the chrome states the metric and its
 * total, with no breadcrumb (there is nowhere to go back to), no lens selector
 * (the root declares no perspectives), and no source disclosure.
 */
export const CanvasRoot: Story = () => <AtPath path={['premium']} theme="light" />
CanvasRoot.storyName = 'Canvas at root'

/**
 * One level in on the "By product" lens: breadcrumb, metric value with its
 * share of the parent, the parent mini-donut with the focused slice, the
 * status chip from `Level.status`, the lens selector with an active pill, the
 * bar chart the level claims via `Level.view`, and the collapsed source-data
 * disclosure from `Level.source`.
 */
export const DrilledLight: Story = () => (
  <AtPath path={drilledPath} perspectiveId="premium/motor/by-product" theme="light" />
)
DrilledLight.storyName = 'Drilled - light'

export const DrilledDark: Story = () => (
  <AtPath path={drilledPath} perspectiveId="premium/motor/by-product" theme="dark" />
)
DrilledDark.storyName = 'Drilled - dark'

/**
 * The same segment through its "Trend" lens: the perspective root owns the
 * data, so the header degrades to the level name (no parent context resolves
 * for a lens root) while the selector marks the active lens and the chart
 * follows the perspective's series semantics.
 */
export const LensSwitched: Story = () => (
  <AtPath
    path={['premium', 'premium/motor', 'premium/motor/trend']}
    perspectiveId="premium/motor/trend"
    theme="light"
  />
)
LensSwitched.storyName = 'Lens switched to trend'

/**
 * A 30rem container at the deepest level: both container queries fire — the
 * parent mini-chart steps aside, the middle of the four-crumb path collapses
 * to an ellipsis, and the metric value drops a size.
 */
export const NarrowContainer: Story = () => <AtPath path={deepPath} theme="light" width="30rem" />
NarrowContainer.storyName = 'Narrow container collapses context'

/**
 * A half-width host at rest beside a neighbor card: the canvas chrome is off,
 * the card keeps its authored span 6, and both cards share the row — the
 * focus-canvas opt-in changes nothing until the panel actually explores.
 */
export const HalfWidthRest: Story = () => (
  <AtPath document={halfWidthDocument} path={[]} theme="light" />
)
HalfWidthRest.storyName = 'Half-width host at rest'

/**
 * The same half-width host while exploring: the grid item expands to the full
 * row (the neighbor reflows below), which gives the chrome its breathing room
 * — the parent mini-donut passes the width query — and the lens selector
 * offers all three sibling perspectives even though the drilled level records
 * only its own ref, exactly as production documents do.
 */
export const HalfWidthExpanded: Story = () => (
  <AtPath
    document={halfWidthDocument}
    path={drilledPath}
    perspectiveId="premium/motor/by-product"
    theme="light"
  />
)
HalfWidthExpanded.storyName = 'Half-width host expands while exploring'
