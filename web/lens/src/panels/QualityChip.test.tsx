import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import type { Availability, Confidence, DashboardDocument, Frame, Panel } from '../contract'
import { DashboardRuntimeProvider, DocumentProvider } from '../runtime'
import { QualityChip, resolveQuality } from './QualityChip'

afterEach(cleanup)

/** Mirrors panels/parity.test.tsx's documentWith/renderDocument template. */
function documentWith(i18n: Record<string, string> = {}): DashboardDocument {
  const panel: Panel = {
    id: 'p', kind: 'stat', title: 'P', semantics: 'series', frame: 'p:root',
    encoding: { label: 'label', value: 'value' },
    format: { value: { kind: 'number', minorUnits: false, precision: 0 } },
    actions: [],
  }
  const frame: Frame = { columns: [{ name: 'label', type: 'string' }, { name: 'value', type: 'number' }], rows: [] }
  return {
    version: '1.0.0',
    snapshotId: 'quality-chip-snapshot',
    meta: { dashboardId: 'quality-chip', title: '', generatedAt: '2026-07-19T00:00:00Z', locale: 'en' },
    layout: { rows: [{ panels: [{ panelId: panel.id, span: 12 }] }] },
    panels: [panel],
    frames: { 'p:root': frame },
    drill: { inlineDepth: 0, edges: {} },
    perspectives: [],
    endpoints: {},
    i18n,
    theme: { palette: {}, series: {} },
  }
}

function renderWithDocument(children: React.ReactNode, i18n: Record<string, string> = {}) {
  return render(
    <div className="lens-root">
      <DocumentProvider initialDocument={documentWith(i18n)}>
        <DashboardRuntimeProvider locale="en">{children}</DashboardRuntimeProvider>
      </DocumentProvider>
    </div>,
  )
}

const CONFIDENCE_LABELS: Record<Confidence, string> = {
  verified: 'Verified',
  calculated: 'Calculated',
  proxy: 'Proxy',
  requires_reconciliation: 'Requires reconciliation',
}

const AVAILABILITY_LABELS: Record<Exclude<Availability, 'available'>, string> = {
  config_required: 'Configuration required',
  empty_source: 'No source data',
  unavailable: 'Unavailable',
}

describe('QualityChip confidence axis', () => {
  it.each(Object.entries(CONFIDENCE_LABELS) as Array<[Confidence, string]>)(
    'renders icon + translated label for confidence %s',
    (confidence, label) => {
      const { container } = render(<QualityChip confidence={confidence} />)
      const chip = container.querySelector('.lens-quality-chip')
      expect(chip).not.toBeNull()
      expect(chip?.querySelector('svg.lens-quality-chip-icon')).not.toBeNull()
      expect(screen.getByText(label)).toBeInTheDocument()
      expect(chip).toHaveClass(`lens-quality-${confidence.replace(/_/g, '-')}`)
      cleanup()
    },
  )
})

describe('QualityChip availability axis', () => {
  it.each(Object.entries(AVAILABILITY_LABELS) as Array<[Exclude<Availability, 'available'>, string]>)(
    'renders icon + translated label for availability %s',
    (availability, label) => {
      const { container } = render(<QualityChip availability={availability} />)
      const chip = container.querySelector('.lens-quality-chip')
      expect(chip).not.toBeNull()
      expect(chip?.querySelector('svg.lens-quality-chip-icon')).not.toBeNull()
      expect(screen.getByText(label)).toBeInTheDocument()
      expect(chip).toHaveClass(`lens-quality-${availability.replace(/_/g, '-')}`)
      cleanup()
    },
  )

  it('renders "available" as no chip at all (available is not a status)', () => {
    const { container } = render(<QualityChip availability="available" />)
    expect(container.querySelector('.lens-quality-chip')).toBeNull()
  })
})

describe('QualityChip precedence', () => {
  it('shows the availability chip when availability is set and not available, even with a confidence value', () => {
    render(<QualityChip confidence="verified" availability="config_required" />)

    expect(screen.getByText('Configuration required')).toBeInTheDocument()
    expect(screen.queryByText('Verified')).toBeNull()
  })

  it('shows the confidence chip when availability is available (or unset)', () => {
    render(<QualityChip confidence="proxy" availability="available" />)

    expect(screen.getByText('Proxy')).toBeInTheDocument()
  })

  it('shows the confidence chip when availability is entirely unset', () => {
    render(<QualityChip confidence="calculated" />)

    expect(screen.getByText('Calculated')).toBeInTheDocument()
  })

  it('renders nothing when neither axis is set', () => {
    const { container } = render(<QualityChip />)

    expect(container.querySelector('.lens-quality-chip')).toBeNull()
    expect(container).toBeEmptyDOMElement()
  })

  it('never renders more than one chip', () => {
    const { container } = render(<QualityChip confidence="verified" availability="unavailable" />)

    expect(container.querySelectorAll('.lens-quality-chip')).toHaveLength(1)
  })
})

describe('resolveQuality', () => {
  it('resolves availability-first when availability is not available', () => {
    const resolved = resolveQuality({ confidence: 'verified', availability: 'empty_source' })
    expect(resolved?.axis).toBe('availability')
    expect(resolved?.value).toBe('empty_source')
  })

  it('resolves confidence when availability is available or unset', () => {
    expect(resolveQuality({ confidence: 'proxy', availability: 'available' })?.axis).toBe('confidence')
    expect(resolveQuality({ confidence: 'proxy' })?.axis).toBe('confidence')
  })

  it('resolves to nothing when both are unset', () => {
    expect(resolveQuality({})).toBeUndefined()
  })
})

describe('QualityChip i18n override', () => {
  it('uses a document-supplied confidence label over the built-in fallback', () => {
    renderWithDocument(<QualityChip confidence="verified" />, { 'confidence.verified': 'Проверено' })

    expect(screen.getByText('Проверено')).toBeInTheDocument()
    expect(screen.queryByText('Verified')).toBeNull()
  })

  it('uses a document-supplied availability label over the built-in fallback', () => {
    renderWithDocument(<QualityChip availability="unavailable" />, { 'availability.unavailable': 'Недоступно' })

    expect(screen.getByText('Недоступно')).toBeInTheDocument()
    expect(screen.queryByText('Unavailable')).toBeNull()
  })
})
