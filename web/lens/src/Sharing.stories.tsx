import type { Story } from '@ladle/react'
import { useEffect, useLayoutEffect } from 'react'
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

function SavedViewsScene() {
  useLayoutEffect(() => {
    const original = window.fetch
    window.fetch = (input) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
      if (url.includes('/schedules')) return Promise.resolve(new Response(JSON.stringify({
        schedules: [{ id: 'schedule-1', viewId: 'view-1', name: 'Weekly claims', cron: '0 8 * * 1', timezone: 'Asia/Tashkent', recipients: ['claims@example.com'] }],
      }), { status: 200 }))
      return Promise.resolve(new Response(JSON.stringify({
        views: [
          { id: 'view-1', dashboardId: 'small', name: 'Team baseline', scope: 'team', stateUrl: '/claims?year=2026', defaultRoleId: 4 },
          { id: 'view-2', dashboardId: 'small', name: 'My investigation', scope: 'personal', stateUrl: '/claims?year=2025&_hidden=paid' },
        ],
        capabilities: { manageTeam: true, scheduleMail: true, roles: [{ id: 4, name: 'Claims analyst' }, { id: 7, name: 'Executive' }] },
      }), { status: 200 }))
    }
    const frame = requestAnimationFrame(() => document.querySelector<HTMLButtonElement>('.lens-saved-views > button')?.click())
    return () => {
      cancelAnimationFrame(frame)
      window.fetch = original
    }
  }, [])
  const document_ = parseDocument({
    ...fixture,
    endpoints: { ...fixture.endpoints, views: '/story/views', schedules: '/story/schedules' },
  })
  return (
    <div className="lens-root lens-story-shell" data-theme="light">
      <DocumentProvider initialDocument={document_}>
        <DashboardRuntimeProvider locale="en"><DashboardPanels /></DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

export const SavedViewsAndSchedule: Story = () => <SavedViewsScene />
