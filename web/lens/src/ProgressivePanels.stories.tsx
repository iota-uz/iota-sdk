import type { Story } from '@ladle/react'
import type { DashboardDocument, Frame, Panel } from './contract'
import { DashboardPanels } from './DashboardPanels'
import { DashboardRuntimeProvider, DocumentProvider } from './runtime'
import './styles.css'

const columns: Frame['columns'] = [
  { name: 'label', type: 'string' },
  { name: 'value', type: 'number' },
]

function stat(id: string, title: string): Panel {
  return {
    id, title, kind: 'stat', semantics: 'series', frame: `panel:${id}`,
    encoding: { label: 'label', value: 'value' }, format: {}, terminal: true, actions: [], deferred: true,
  }
}

const panels = [stat('ready', 'Готовая панель'), stat('loading', 'Долгий расчёт'), stat('error', 'Ошибка панели')]
const progressiveDocument: DashboardDocument = {
  version: '1.0.0', snapshotId: 'progressive-story',
  meta: { dashboardId: 'progressive', title: 'Независимые панели', generatedAt: '2026-07-31T10:00:00Z', locale: 'ru' },
  header: { title: 'Независимые панели', subtitle: 'Один сбой не блокирует остальные' },
  layout: { rows: [{ panels: panels.map(({ id }) => ({ panelId: id, span: 4 })) }] },
  panels, frames: {}, drill: { inlineDepth: 0, edges: {} }, perspectives: [],
  endpoints: { panel: '/lens/panel' }, i18n: {}, theme: { palette: {}, series: {} },
}

const fetcher: typeof fetch = async (_input, init) => {
  const request = JSON.parse(typeof init?.body === 'string' ? init.body : '{}') as { panelId?: string }
  if (request.panelId === 'loading') return new Promise<Response>(() => undefined)
  if (request.panelId === 'error') {
    return new Response(JSON.stringify({ error: 'internal', message: 'Расчёт не выполнен' }), {
      status: 500, headers: { 'Content-Type': 'application/json' },
    })
  }
  return new Response(JSON.stringify({
    frames: { 'panel:ready': { columns, rows: [['Готово', 42]] } },
    calculation: { durationMs: 840, cacheHit: true, calculatedAt: '2026-07-31T10:00:00Z' },
  }), { status: 200, headers: { 'Content-Type': 'application/json' } })
}

export const SiblingIsolation: Story = () => (
  <div className="lens-root" data-theme="light" lang="ru">
    <DocumentProvider initialDocument={progressiveDocument} fetcher={fetcher}>
      <DashboardRuntimeProvider fetcher={fetcher} locale="ru">
        <DashboardPanels />
      </DashboardRuntimeProvider>
    </DocumentProvider>
  </div>
)
SiblingIsolation.storyName = 'Sibling ready, loading, and error'
