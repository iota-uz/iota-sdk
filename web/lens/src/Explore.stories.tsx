import type { Story } from '@ladle/react'
import { useEffect, useState } from 'react'
import fixture from '../fixtures/explore.json'
import { parseDocument, type FieldFormat, type Level, type Node, type Perspective } from './contract'
import { DashboardPanels } from './DashboardPanels'
import { DrillOverlay } from './explore/DrillOverlay'
import type { DrillTarget } from './explore/model'
import { DashboardRuntimeProvider, DocumentProvider, navigationToURL, useDrill } from './runtime'
import './styles.css'

const dashboardDocument = parseDocument(fixture)
const branchKey = 'profitability/operating-margin'

function OpenBranch() {
  const { drillInto } = useDrill()
  useEffect(() => drillInto(branchKey, 'margin'), [drillInto])
  return null
}

function Walkthrough({ startAtBranch = false, children }: { startAtBranch?: boolean; children: React.ReactNode }) {
  return (
    <div className="lens-root lens-explore-story">
      <aside className="lens-story-guide">{children}</aside>
      <DocumentProvider initialDocument={dashboardDocument}>
        <DashboardRuntimeProvider locale="en">
          {startAtBranch && <OpenBranch />}
          <DashboardPanels />
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

export const FullDrillFlow: Story = () => (
  <Walkthrough>
    Select <strong>Operating margin</strong>, choose <strong>Composition</strong>, then move through
    <strong> Services</strong> and <strong>Sales</strong>. Use the path rail to jump back to any level.
  </Walkthrough>
)
FullDrillFlow.storyName = 'Full drill flow - three levels'

export const PerspectiveSwitching: Story = () => (
  <Walkthrough startAtBranch>
    The perspective set belongs to the active <strong>Operating margin</strong> segment. Compare Composition,
    Trend, Bridge, and Evidence without leaving the panel.
  </Walkthrough>
)
PerspectiveSwitching.storyName = 'Perspective switching on a segment'

/**
 * Clicks a button by accessible name once the panel has mounted, so a story can
 * present a transient surface (the drill overlay, an expanded panel) as a
 * stable screenshot.
 */
function ClickOnMount({ labels }: { labels: Array<string> }) {
  useEffect(() => {
    let cancelled = false
    const press = (index: number) => {
      if (cancelled || index >= labels.length) return
      const button = window.document.querySelector<HTMLButtonElement>(`button[aria-label="${labels[index]}"]`)
      button?.click()
      window.requestAnimationFrame(() => press(index + 1))
    }
    window.requestAnimationFrame(() => press(0))
    return () => { cancelled = true }
  }, [labels])
  return null
}

function OverlayStory({ theme, labels }: { theme: 'light' | 'dark'; labels: Array<string> }) {
  return (
    <div className="lens-root" data-theme={theme}>
      <DocumentProvider initialDocument={dashboardDocument}>
        <DashboardRuntimeProvider locale="en">
          <DashboardPanels />
          <ClickOnMount labels={labels} />
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

export const DrillOverlayLight: Story = () => <OverlayStory theme="light" labels={['Show breakdown']} />
DrillOverlayLight.storyName = 'Drill overlay - light'

export const DrillOverlayDark: Story = () => <OverlayStory theme="dark" labels={['Show breakdown']} />
DrillOverlayDark.storyName = 'Drill overlay - dark'

export const DrillOverlayInExpandedPanel: Story = () => (
  <OverlayStory theme="light" labels={['Expand panel', 'Show breakdown']} />
)
DrillOverlayInExpandedPanel.storyName = 'Drill overlay inside an expanded panel'

/**
 * The worst case for the header trail: the deepest path inside a third-width
 * card whose header also carries a total badge. The header keeps the back
 * button and the current level (or, when even that cannot stay readable,
 * only the back button); the full path lives in the overlay.
 */
const deepPath = [
  'profitability',
  'profitability/operating-margin',
  'profitability/operating-margin/composition',
  'profitability/operating-margin/composition/transactions',
]

const narrowDocument = {
  ...dashboardDocument,
  panels: dashboardDocument.panels.map((panel) => ({ ...panel, total: 1_840_000 })),
  layout: { rows: [{ panels: dashboardDocument.layout.rows[0]!.panels.map((item) => ({ ...item, span: 3 })) }] },
}

function narrowDocumentWithSpan(span: number) {
  return {
    ...narrowDocument,
    layout: { rows: [{ panels: narrowDocument.layout.rows[0]!.panels.map((item) => ({ ...item, span })) }] },
  }
}

function AtDeepestLevel({ theme, labels = [], span = 3 }: {
  theme: 'light' | 'dark'
  labels?: Array<string>
  span?: number
}) {
  useState(() => {
    window.history.replaceState(null, '', navigationToURL(
      { panelId: narrowDocument.panels[0]!.id, path: deepPath, perspectiveId: 'profitability/operating-margin/composition' },
      new URL(window.location.href),
    ))
    return true
  })
  return (
    <div className="lens-root" data-theme={theme}>
      <DocumentProvider initialDocument={narrowDocumentWithSpan(span)}>
        <DashboardRuntimeProvider locale="en">
          <DashboardPanels />
          {labels.length > 0 && <ClickOnMount labels={labels} />}
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

export const NarrowDeepTrailLight: Story = () => <AtDeepestLevel theme="light" />
NarrowDeepTrailLight.storyName = 'Narrow card, deepest path - light'

export const NarrowDeepTrailDark: Story = () => <AtDeepestLevel theme="dark" />
NarrowDeepTrailDark.storyName = 'Narrow card, deepest path - dark'

/**
 * Narrower still: the header can no longer hold a readable level name, so it
 * steps aside and leaves the back button and the breakdown affordance rather
 * than a one-letter stump.
 */
export const NarrowestTrail: Story = () => <AtDeepestLevel theme="light" span={2} />
NarrowestTrail.storyName = 'Header too narrow for a level name'

/**
 * A fork in the drill path: the level owns no data of its own, so the panel
 * asks for a view instead of keeping the parent level's numbers on screen
 * under the child's title.
 */
function AtFork({ theme }: { theme: 'light' | 'dark' }) {
  useState(() => {
    window.history.replaceState(null, '', navigationToURL(
      {
        panelId: dashboardDocument.panels[0]!.id,
        path: ['profitability', 'profitability/operating-margin'],
      },
      new URL(window.location.href),
    ))
    return true
  })
  return (
    <div className="lens-root" data-theme={theme}>
      <DocumentProvider initialDocument={dashboardDocument}>
        <DashboardRuntimeProvider locale="en"><DashboardPanels /></DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

export const AwaitingPerspectiveLight: Story = () => <AtFork theme="light" />
AwaitingPerspectiveLight.storyName = 'Level fork awaits a view - light'

export const AwaitingPerspectiveDark: Story = () => <AtFork theme="dark" />
AwaitingPerspectiveDark.storyName = 'Level fork awaits a view - dark'

/**
 * The contextual card as it stands when a chart segment is clicked: the
 * statistics header (color swatch, value, share of the total), the promoted
 * expansion, the quiet copy control, the perspective pills, and the caret that
 * ties it back to the mark. Rendered directly with a fixed anchor so the whole
 * segment surface is a stable screenshot without steering a canvas click.
 */
const segmentPerspective = (id: string, label: string): Perspective => ({
  id,
  explorerId: 'profitability',
  branchKey: 'profitability/operating-margin',
  key: id,
  label,
  semantics: 'partition',
  root: 'profitability/operating-margin',
})

const segmentTarget: DrillTarget = {
  node: { key: 'profitability/services', path: ['profitability', 'profitability/services'], label: 'Services' } as Node,
  label: 'Services',
  value: 1_284_000,
  share: 0.698,
  total: 1_840_000,
  target: {} as Level,
  expandsToFork: false,
  breakdown: [],
  perspectives: [
    segmentPerspective('composition', 'Composition'),
    segmentPerspective('trend', 'Trend'),
  ],
}

const segmentValueFormat: FieldFormat = { kind: 'money', currency: 'USD', minorUnits: false, precision: 0, symbol: '$' }

function SegmentOverlay({ theme }: { theme: 'light' | 'dark' }) {
  return (
    <div className="lens-root" data-theme={theme} style={{ minHeight: '100vh' }}>
      <DocumentProvider initialDocument={dashboardDocument}>
        <DashboardRuntimeProvider locale="en">
          <DrillOverlay
            accentColor="#7c3aed"
            anchor={{ x: 420, y: 360 }}
            dark={theme === 'dark'}
            onClose={() => {}}
            onDrillChild={() => {}}
            onDrillInto={() => {}}
            onPerspective={() => {}}
            target={segmentTarget}
            theme={theme}
            valueFormat={segmentValueFormat}
          />
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

export const SegmentOverlayLight: Story = () => <SegmentOverlay theme="light" />
SegmentOverlayLight.storyName = 'Segment overlay statistics - light'

export const SegmentOverlayDark: Story = () => <SegmentOverlay theme="dark" />
SegmentOverlayDark.storyName = 'Segment overlay statistics - dark'

export const KeyboardWalkthrough: Story = () => (
  <Walkthrough>
    Press <strong>Tab</strong> to reach a segment, use <strong>arrow keys</strong> between siblings, and press
    <strong> Enter</strong> or <strong>Space</strong> to explore. Press <strong>Escape</strong> to go back.
  </Walkthrough>
)
KeyboardWalkthrough.storyName = 'Keyboard walkthrough'
