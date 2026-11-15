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
  })
})
