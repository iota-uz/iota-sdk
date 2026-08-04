import type { Story } from '@ladle/react'
import { useEffect } from 'react'
import fixture from '../fixtures/small.json'
import { parseDocument } from './contract'
import { DashboardPanels } from './DashboardPanels'
import { DashboardRuntimeProvider, DocumentProvider } from './runtime'
import './styles.css'

function SharingScene({ openPanelMenu = false }: { openPanelMenu?: boolean }) {
  useEffect(() => {
    if (!openPanelMenu) return undefined
    const frame = requestAnimationFrame(() => {
      document.querySelector<HTMLButtonElement>('button[aria-label="Export panel"]')?.click()
    })
    return () => cancelAnimationFrame(frame)
  }, [openPanelMenu])
  const document_ = parseDocument({ ...fixture, endpoints: { ...fixture.endpoints, export: '/story/export' } })
  return (
    <div className="lens-root lens-story-shell" data-theme="light">
      <DocumentProvider initialDocument={document_}>
        <DashboardRuntimeProvider locale="en">
          <DashboardPanels />
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

export const SliceLink: Story = () => <SharingScene />
export const PanelImageFormats: Story = () => <SharingScene openPanelMenu />
