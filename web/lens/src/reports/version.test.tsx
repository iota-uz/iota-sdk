import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library'
import { createSignal } from 'solid-js'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../../fixtures/reusable-reports.json'
import reopened from '../../fixtures/reusable-reports-reopened.json'
import { LensDashboard } from '../LensDashboard'
import { usePrint } from '../runtime'
import { parseDocument } from '../contract'
import type { EffectiveContext } from '../contract/types'
import type { ReportPlacementHost } from './placementHost'
import { compareReportVersion, reportPlacementActions, withoutUnavailablePlacements, type ReportRevision, type ReportVersionStatus } from './version'

afterEach(() => {
  cleanup()
  document.body.replaceChildren()
})

const opened: ReportRevision = {
  definition: { revision: '1', source: { id: 'monthly', version: 1 }, view: 'summary' },
  source: { id: 'monthly', version: 1, revision: '1', capabilities: { views: ['summary'] } },
}
const latest = (change: (value: ReportRevision) => void): ReportRevision => {
  const value = structuredClone(opened)
  change(value)
  return value
}

// False green: a status computed from revisions alone would offer an update whose contract the source no longer runs.
describe('compareReportVersion', () => {
  it('separates compatible updates from incompatible contracts and views', () => {
    expect(compareReportVersion(opened, opened)).toBe('current')
    expect(compareReportVersion(opened, latest((value) => { value.definition.revision = '2' }))).toBe('update-available')
    expect(compareReportVersion(opened, latest((value) => { value.source.revision = '2' }))).toBe('update-available')
    expect(compareReportVersion(opened, latest((value) => { value.source.version = 2 }))).toBe('incompatible')
    expect(compareReportVersion(opened, latest((value) => { value.definition.view = 'detail' }))).toBe('incompatible')
    expect(compareReportVersion(opened, latest((value) => { value.definition.revision = '2'; value.definition.source.version = 2; value.source.version = 2 }))).toBe('update-available')
  })
})

// False green: reading the capability flags alone would offer drill or export on a context the source cannot carry.
describe('reportPlacementActions', () => {
  const context = (overrides: Partial<EffectiveContext>) => ({ conflicts: [], unsupported: [], ...overrides })
  it('offers only operations the source declares and the context permits', () => {
    expect(reportPlacementActions({ drill: true, save: false, export: true })).toEqual({ drill: true, save: false, export: true })
    expect(reportPlacementActions({ drill: true, save: true, export: true }, context({ unsupportedOperations: { export: [{ dimension: 'product', values: ['A'], origin: { scope: 'dashboard', id: 'd' } }] } })))
      .toEqual({ drill: true, save: true, export: false })
    expect(reportPlacementActions({ drill: true, save: true, export: true }, context({ conflicts: ['product'] }))).toEqual({ drill: false, save: false, export: false })
  })
})

const region = (name: string) => screen.getAllByRole('region', { name })[0]!
const documentFixture = () => parseDocument(structuredClone(fixture.document))

function mount(host: ReportPlacementHost, fetcher: typeof fetch = () => new Promise<Response>(() => undefined), src?: string) {
  return render(() => <LensDashboard initialDocument={documentFixture()} src={src} fetcher={fetcher} reportPlacementHost={host} />)
}

describe('report placement version notice', () => {
  // False green: a notice that replaced the figures would satisfy "offers an update" while discarding the opened result.
  it('keeps the opened figures beside an update offer and dismisses it on keep', () => {
    const keep = vi.fn()
    mount({ status: (id) => id === 'first' ? 'update-available' : 'current', keep })
    const first = region('First six months')
    const notice = within(first).getByTestId('report-version-first')
    expect(notice).toHaveTextContent('A newer version of this report is available.')
    expect(within(first).getAllByText('Online').length).toBeGreaterThan(0)
    expect(within(region('Revenue')).queryByTestId('report-version-second')).toBeNull()
    fireEvent.click(within(notice).getByRole('button', { name: 'Keep this version' }))
    expect(keep).toHaveBeenCalledWith('first')
    expect(within(first).queryByTestId('report-version-first')).toBeNull()
    expect(document.activeElement).toBe(first)
    expect(within(first).getAllByText('Online').length).toBeGreaterThan(0)
  })

  // False green: a host acknowledgement without a document refetch would leave the old figures labelled as updated.
  it('refreshes the document after the host adopts the update and retains figures when it fails', async () => {
    const [status, setStatus] = createSignal<ReportVersionStatus>('update-available')
    let reject = true
    const update = vi.fn<(id: string) => Promise<void>>(() => {
      if (reject) return Promise.reject(new Error('rejected'))
      setStatus('current')
      return Promise.resolve()
    })
    const fetcher = vi.fn<typeof fetch>(() => Promise.resolve(Response.json({ ...fixture.document, snapshotId: 'updated' })))
    mount({ status: (id) => id === 'first' ? status() : 'current', update }, fetcher, '/reports/document')
    const first = region('First six months')
    fireEvent.click(within(first).getByRole('button', { name: 'Update report' }))
    await waitFor(() => expect(within(first).getByRole('alert')).toHaveTextContent('The report could not be updated'))
    expect(fetcher).not.toHaveBeenCalled()
    expect(within(first).getAllByText('Online').length).toBeGreaterThan(0)

    reject = false
    fireEvent.click(within(first).getByRole('button', { name: 'Update report' }))
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1))
    expect(update).toHaveBeenCalledTimes(2)
    expect(update).toHaveBeenLastCalledWith('first')
    await waitFor(() => expect(screen.queryByTestId('report-version-first')).toBeNull())
  })

  // False green: gating the notice on host status alone would unmount it when the host adopts the update, hiding a failed refresh.
  it('keeps the failure and retry visible when the host adopts the update but the refresh fails', async () => {
    const [status, setStatus] = createSignal<ReportVersionStatus>('update-available')
    const update = vi.fn<(id: string) => Promise<void>>(() => {
      setStatus('current')
      return Promise.resolve()
    })
    const fetcher = vi.fn<typeof fetch>(() => Promise.reject(new TypeError('offline')))
    mount({ status: (id) => id === 'first' ? status() : 'current', update }, fetcher, '/reports/document')
    const first = region('First six months')
    fireEvent.click(within(first).getByRole('button', { name: 'Update report' }))
    await waitFor(() => expect(within(first).getByRole('alert')).toHaveTextContent('The report could not be updated'))
    expect(within(first).getByRole('button', { name: 'Update report' })).toBeInTheDocument()
  })

  // False green: hiding every placement would also pass; only the incompatible one becomes unavailable.
  it('explains an incompatible placement instead of showing its figures', () => {
    mount({ status: (id) => id === 'first' ? 'incompatible' : 'current' })
    const first = region('First six months')
    expect(within(first).getByRole('alert')).toHaveTextContent('This report is unavailable')
    expect(within(first).queryByText('Online')).toBeNull()
    expect(within(first).queryByRole('button')).toBeNull()
    expect(within(region('Revenue')).getAllByText('Online').length).toBeGreaterThan(0)
  })
})

describe('reopened composition with an incompatible placement', () => {
  // False green: a fixture authored in TS would not prove the Go composer emits the wire marker; this one is generated by cmd/lens-report-example --reopen.
  it('renders the current placement and explains the unavailable one without a host', () => {
    render(() => <LensDashboard initialDocument={parseDocument(structuredClone(reopened.document))} fetcher={() => new Promise<Response>(() => undefined)} />)
    const first = region('First six months')
    expect(within(first).getByRole('alert')).toHaveTextContent('This report is unavailable')
    expect(within(first).queryByRole('table')).toBeNull()
    expect(within(first).queryByRole('button', { name: 'Update report' })).toBeNull()
    expect(within(region('Revenue')).getAllByText('Online').length).toBeGreaterThan(0)
  })
})

function PrintProbe() {
  const print = usePrint()
  return <>
    <button type="button" onClick={() => void print.run({ preview: true })}>Preview print</button>
    <output data-testid="print-sections">{print.report ? JSON.stringify(print.report.sections.map((section) => section.panel.id)) : ''}</output>
    <output data-testid="print-warnings">{print.report?.warnings.join('\n') ?? ''}</output>
  </>
}

describe('print of unavailable placements', () => {
  const printed = async (document: ReturnType<typeof parseDocument>, host?: ReportPlacementHost) => {
    render(() => <LensDashboard initialDocument={document} fetcher={() => new Promise<Response>(() => undefined)} reportPlacementHost={host} toolbar={() => <PrintProbe />} />)
    fireEvent.click(screen.getByRole('button', { name: 'Preview print' }))
    await waitFor(() => expect(screen.getByTestId('print-sections').textContent).not.toBe(''))
    return { sections: JSON.parse(screen.getByTestId('print-sections').textContent!) as string[], warnings: screen.getByTestId('print-warnings').textContent ?? '' }
  }

  // False green: hiding figures on screen only would still print the incompatible placement's numbers.
  it('prints no figures for a placement the host reports incompatible and says why', async () => {
    const { sections, warnings } = await printed(documentFixture(), { status: (id) => id === 'first' ? 'incompatible' : 'current' })
    expect(sections.some((id) => id.startsWith('first::'))).toBe(false)
    expect(sections.some((id) => id.startsWith('second::'))).toBe(true)
    expect(warnings).toContain('First six months: This report is unavailable')
  })

  // False green: the host-driven case alone would miss the wire marker a reopened composition carries.
  it('prints no figures for a placement the reopened document marks unavailable', async () => {
    const { sections, warnings } = await printed(parseDocument(structuredClone(reopened.document)))
    expect(sections.some((id) => id.startsWith('first::'))).toBe(false)
    expect(sections.some((id) => id.startsWith('second::'))).toBe(true)
    expect(warnings).toContain('First six months: This report is unavailable')
  })

  it('leaves a document without unavailable placements untouched', () => {
    const document = documentFixture()
    expect(withoutUnavailablePlacements(document, () => false)).toEqual({ document, withheld: [] })
  })
})
