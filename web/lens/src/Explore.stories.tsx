import type { Story } from '@ladle/react'
import { useEffect } from 'react'
import fixture from '../fixtures/explore.json'
import { parseDocument } from './contract'
import { DashboardPanels } from './DashboardPanels'
import { DashboardRuntimeProvider, DocumentProvider, useDrill } from './runtime'
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
FullDrillFlow.storyName = 'Full drill flow · three levels'

export const PerspectiveSwitching: Story = () => (
  <Walkthrough startAtBranch>
    The perspective set belongs to the active <strong>Operating margin</strong> segment. Compare Composition,
    Trend, Bridge, and Evidence without leaving the panel.
  </Walkthrough>
)
PerspectiveSwitching.storyName = 'Perspective switching on a segment'

export const KeyboardWalkthrough: Story = () => (
  <Walkthrough>
    Press <strong>Tab</strong> to reach a segment, use <strong>arrow keys</strong> between siblings, and press
    <strong> Enter</strong> or <strong>Space</strong> to explore. Press <strong>Escape</strong> to go back.
  </Walkthrough>
)
KeyboardWalkthrough.storyName = 'Keyboard walkthrough'
