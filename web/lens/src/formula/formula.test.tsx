import { render, screen, waitFor, fireEvent } from '@solidjs/testing-library'
import { describe, expect, it, vi } from 'vitest'
import type { Formula, Frame, Panel } from '../contract'
import { FormulaView } from './FormulaView'
import { PanelFormula } from './PanelFormula'
import { formulaValueLine, resolveFormulaVariables } from './values'

const formula: Formula = {
  latex: String.raw`\frac{\htmlData{var=N}{N}}{E}`,
  alt: 'N divided by E',
  variables: [
    { key: 'N', label: 'Net profit', ref: { kind: 'field', name: 'net' }, source: 'GL' },
    { key: 'E', label: 'Equity', ref: { kind: 'field', name: 'missing' } },
  ],
}
const panel = {
  id: 'roe', kind: 'stat', title: 'ROE', semantics: 'series', frame: 'f',
  encoding: { id: 'id', value: 'roe' }, format: {}, actions: [], formula,
} as unknown as Panel
const frame: Frame = { columns: [{ name: 'roe', type: 'number' }, { name: 'net', type: 'number' }], rows: [[12.5, 300]] }

describe('formula values', () => {
  it('reads variables from the figure frame, same row', () => {
    const vars = resolveFormulaVariables(formula, panel, frame, 'en')
    expect(vars[0]?.value).toBe('300')
    expect(vars[1]?.value).toBeUndefined()
    expect(formulaValueLine(panel, frame, 'en')).toBe('12.5')
  })

  it('formats a variable read from another column with its own format', () => {
    // A stat declares a format only for its own figure. The other KPIs of the
    // row its formula names would otherwise print as bare numbers.
    const withFormat: Formula = {
      latex: 'a',
      variables: [{ key: 'N', label: 'Net profit', ref: { kind: 'field', name: 'net' }, format: { kind: 'money', currency: 'UZS', precision: 0, minorUnits: false } }],
    }
    expect(resolveFormulaVariables(withFormat, panel, frame, 'en')[0]?.value).toBe('UZS\u00a0300')
  })

  it('keeps the unabbreviated value of a compact figure for the variable list', () => {
    // «4.2B − 1.1B = 3.0B» reads as wrong arithmetic; the list states the
    // operands in full so a reader can check the sum.
    const compact: Formula = {
      latex: 'a',
      variables: [{ key: 'N', label: 'Net', ref: { kind: 'field', name: 'net' }, format: { kind: 'number', compact: true, minorUnits: false } }],
    }
    const big: Frame = { columns: [{ name: 'net', type: 'number' }], rows: [[4213507880]] }
    const [view] = resolveFormulaVariables(compact, panel, big, 'en')
    expect(view?.value).not.toBe(view?.exact)
    expect(view?.exact).toBe('4,213,507,880')
  })

  it('says why a field cannot be stated when its row says so', () => {
    const gated: Formula = {
      latex: 'a',
      variables: [{ key: 'N', label: 'Net', value: '99', ref: { kind: 'field', name: 'net', availability: 'net_state' } }],
    }
    const row: Frame = { columns: [{ name: 'net', type: 'number' }, { name: 'net_state', type: 'string' }], rows: [[null, 'config_required']] }
    const [view] = resolveFormulaVariables(gated, panel, row, 'en')
    expect(view?.value).toBeUndefined()
    expect(view?.availability).toBe('config_required')
  })
})

describe('FormulaView', () => {
  it('shows a variable value on focus and hides on blur', async () => {
    const vars = resolveFormulaVariables(formula, panel, frame, 'en')
    const { container } = render(() => <FormulaView formula={formula} variables={vars} />)
    await waitFor(() => expect(container.querySelector('[data-var="N"]')).not.toBeNull())
    const symbol = container.querySelector<HTMLElement>('[data-var="N"]')!
    expect(symbol.tabIndex).toBe(0)
    symbol.dispatchEvent(new FocusEvent('focus'))
    await waitFor(() => expect(container.querySelector('[role="tooltip"]')).not.toBeNull())
    expect(container.querySelector('[role="tooltip"]')).toHaveTextContent('Net profit')
    expect(container.querySelector('[role="tooltip"]')).toHaveTextContent('300')
    symbol.dispatchEvent(new FocusEvent('blur'))
    await waitFor(() => expect(container.querySelector('[role="tooltip"]')).toBeNull())
    expect(container.querySelector('[role="math"]')).toHaveAttribute('aria-label', 'N divided by E')
  })

  it('keeps the variable tip under its symbol when the symbol moves while shown', async () => {
    // KaTeX's faces arrive after the symbol is focused and move it; a tip
    // measured once on focus would stay where the fallback glyph was.
    const fonts = new EventTarget()
    const previous = Object.getOwnPropertyDescriptor(document, 'fonts')
    Object.defineProperty(document, 'fonts', { configurable: true, value: fonts })
    try {
      const vars = resolveFormulaVariables(formula, panel, frame, 'en')
      const { container } = render(() => <FormulaView formula={formula} variables={vars} />)
      await waitFor(() => expect(container.querySelector('[data-var="N"]')).not.toBeNull())
      const symbol = container.querySelector<HTMLElement>('[data-var="N"]')!
      let rect = { left: 100, width: 20, bottom: 50 }
      symbol.getBoundingClientRect = () => ({ ...rect, right: rect.left + rect.width, top: rect.bottom - 20, height: 20, x: rect.left, y: rect.bottom - 20, toJSON: () => rect }) as DOMRect
      symbol.dispatchEvent(new FocusEvent('focus'))
      const tip = () => container.querySelector<HTMLElement>('[role="tooltip"]')
      await waitFor(() => expect(tip()).not.toBeNull())
      expect(tip()!.style.left).toBe('110px')
      expect(tip()!.style.top).toBe('56px')

      rect = { left: 104, width: 22, bottom: 52 }
      fonts.dispatchEvent(new Event('loadingdone'))
      await waitFor(() => expect(tip()!.style.left).toBe('115px'))
      expect(tip()!.style.top).toBe('58px')

      rect = { left: 40, width: 22, bottom: 30 }
      globalThis.dispatchEvent(new Event('scroll'))
      await waitFor(() => expect(tip()!.style.left).toBe('51px'))

      symbol.dispatchEvent(new FocusEvent('blur'))
      await waitFor(() => expect(tip()).toBeNull())
    } finally {
      if (previous) Object.defineProperty(document, 'fonts', previous)
      else delete (document as { fonts?: unknown }).fonts
    }
  })

  it('reports invalid LaTeX and keeps rendering', async () => {
    const log = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    const onError = vi.fn()
    const { container } = render(() => <FormulaView formula={{ latex: String.raw`\frac{a}` }} onError={onError} />)
    await waitFor(() => expect(container.querySelector('[role="alert"]')).toHaveTextContent('could not be displayed'))
    expect(onError).toHaveBeenCalledTimes(1)
    expect(log).toHaveBeenCalled()
  })

  it('treats a server-flagged error as an error without rendering', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined)
    const { container } = render(() => <FormulaView formula={{ latex: 'x', error: true }} />)
    await waitFor(() => expect(container.querySelector('[role="alert"]')).not.toBeNull())
  })
})

describe('PanelFormula', () => {
  it('renders no ƒ without a formula', () => {
    render(() => <PanelFormula frame={frame} panel={{ ...panel, formula: undefined } as Panel} />)
    expect(screen.queryByTestId('lens-formula-button')).toBeNull()
  })

  it('opens a preview with the value line from the frame', async () => {
    render(() => <PanelFormula frame={frame} panel={panel} />)
    const button = screen.getByTestId('lens-formula-button')
    fireEvent.focus(button)
    expect(await screen.findByTestId('lens-formula-value')).toHaveTextContent('ROE = 12.5')
  })

  it('tells a hover reader that a click lists the variables', async () => {
    render(() => <PanelFormula frame={frame} panel={panel} />)
    const button = screen.getByTestId('lens-formula-button')
    fireEvent.focus(button)
    expect(await screen.findByTestId('lens-formula-hint')).toBeInTheDocument()
    fireEvent.click(button)
    await waitFor(() => expect(screen.queryByTestId('lens-formula-hint')).toBeNull())
    expect(screen.getByText('Equity').nextElementSibling).toHaveTextContent('no data')
  })

  it('says the panel note inside the formula', async () => {
    render(() => <PanelFormula frame={frame} note="Net of reinsurance." panel={panel} />)
    fireEvent.focus(screen.getByTestId('lens-formula-button'))
    expect(await screen.findByTestId('lens-formula-note')).toHaveTextContent('Net of reinsurance.')
  })

  it('describes the trigger with the note it replaced', () => {
    render(() => <PanelFormula frame={frame} note="Net of reinsurance." panel={panel} />)
    // The ⓘ named the note on focus; the ƒ must not make it silent.
    expect(screen.getByTestId('lens-formula-button')).toHaveAccessibleDescription('Net of reinsurance.')
  })
})

describe('value rules', () => {
  const flow = {
    ...panel, kind: 'metric_flow', encoding: { id: 'id', value: 'amount' },
    metricFlow: { stages: [
      { key: 'rev', label: 'Revenue', role: 'input' },
      { key: 'net', label: 'Net', role: 'result' },
      { key: 'gone', label: 'Gone', role: 'input', availability: 'unavailable' },
    ] },
    formula: { latex: 'a', variables: [
      { key: 'N', label: 'Net', ref: { kind: 'stage', name: 'net' } },
      { key: 'G', label: 'Gone', ref: { kind: 'stage', name: 'gone' } },
    ] },
  } as unknown as Panel
  const flowFrame: Frame = {
    columns: [{ name: 'id', type: 'string' }, { name: 'amount', type: 'number' }],
    rows: [['rev', 100], ['net', 30], ['gone', 0]],
  }

  it('states a flow by its result stage and dashes unavailable stages', () => {
    expect(formulaValueLine(flow, flowFrame, 'en')).toBe('30')
    const vars = resolveFormulaVariables(flow.formula!, flow, flowFrame, 'en')
    expect(vars[0]?.value).toBe('30')
    expect(vars[1]?.value).toBeUndefined()
    expect(vars[1]?.availability).toBe('unavailable')
  })
})

describe('supplied values', () => {
  const stageFormula: Formula = { latex: 'a', variables: [{ key: 'G', label: 'Gone', value: '99', ref: { kind: 'stage', name: 'gone' } }] }
  const base = {
    ...panel, kind: 'metric_flow', encoding: { id: 'id', value: 'amount' },
    metricFlow: { stages: [{ key: 'gone', label: 'Gone', role: 'input', availability: 'unavailable' }] },
    formula: stageFormula,
  } as unknown as Panel
  const f: Frame = { columns: [{ name: 'id', type: 'string' }, { name: 'amount', type: 'number' }], rows: [['gone', 0]] }

  it('clears a supplied value for an unavailable stage but keeps it without a frame', () => {
    expect(resolveFormulaVariables(stageFormula, base, f, 'en')[0]?.value).toBeUndefined()
    expect(resolveFormulaVariables(stageFormula, base, undefined, 'en')[0]?.value).toBe('99')
  })
})

describe('<lens-formula>', () => {
  it('says the note inside the ƒ', async () => {
    const { registerLensFormulaElement } = await import('./element')
    registerLensFormulaElement()
    const host = document.createElement('lens-formula')
    host.setAttribute('formula', JSON.stringify({ latex: 'a' }))
    host.setAttribute('label', 'ROE')
    host.setAttribute('note', 'Net of reinsurance.')
    document.body.append(host)
    fireEvent.focus(host.querySelector('[data-testid="lens-formula-button"]')!)
    expect(await screen.findByTestId('lens-formula-note')).toHaveTextContent('Net of reinsurance.')
    host.remove()
  })
})
