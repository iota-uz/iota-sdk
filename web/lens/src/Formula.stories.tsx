import { onMount, type JSX } from 'solid-js'
import type { Formula, Panel } from './contract'
import { FormulaVariables, FormulaView } from './formula/FormulaView'
import { PanelFormula } from './formula/PanelFormula'
import type { FormulaVariableView } from './formula/values'
import './styles.css'

/**
 * A figure's methodology: the typeset formula, the variables whose values are
 * those of the slice on screen, and the states around them — loading, a markup
 * error that must not break the report, and a figure with no formula at all.
 */

const formula: Formula = {
  latex: String.raw`\mathrm{ROE} = \frac{\htmlData{var=N}{N}}{\htmlData{var=E}{E_{\mathrm{avg}}}} \cdot 100\%`,
  alt: 'Return on equity equals net profit divided by average equity, times one hundred percent',
  markup: '<math display="block"><mrow><mtext>ROE</mtext><mo>=</mo><mfrac><mi>N</mi><mi>E</mi></mfrac></mrow></math>',
  version: '3',
  date: '2026-09-01',
  variables: [
    { key: 'N', label: 'Net profit', source: 'General ledger, account 9910' },
    { key: 'E', label: 'Average equity', source: 'Balance sheet, section III' },
  ],
}

const variables: FormulaVariableView[] = [
  { key: 'N', label: 'Net profit', source: 'General ledger, account 9910', value: '4.2 bn UZS' },
  { key: 'E', label: 'Average equity', source: 'Balance sheet, section III', value: '33.6 bn UZS' },
]

function Card(props: { label: string; children: JSX.Element }): JSX.Element {
  return (
    <div class="lens-story-cell" style={{ 'max-width': '26rem' }}>
      <span class="lens-story-cell-label">{props.label}</span>
      <div class="lens-info-tip-bubble" style={{ position: 'static', width: '100%', 'max-width': 'none' }}>
        {props.children}
      </div>
    </div>
  )
}

const panel = (withFormula: boolean): Panel => ({
  id: 'roe',
  kind: 'stat',
  title: 'ROE',
  semantics: 'series',
  frame: 'roe',
  encoding: { id: 'id', value: 'value' },
  format: {},
  actions: [],
  ...(withFormula ? { formula } : {}),
}) as Panel

function Matrix(props: { theme: 'light' | 'dark' }): JSX.Element {
  // Hover is a pointer state; the story reaches it the way Tab does.
  let hoverCell: HTMLDivElement | undefined
  onMount(() => {
    const id = globalThis.setInterval(() => {
      const target = hoverCell?.querySelector<HTMLElement>('[data-var="N"]')
      if (!target) return
      globalThis.clearInterval(id)
      target.focus()
    }, 100)
  })
  return (
    <div class="lens-root lens-story-matrix" data-theme={props.theme} style={{ display: 'grid', gap: '16px', 'grid-template-columns': 'repeat(2, minmax(0, 1fr))' }}>
      <Card label="formula">
        <FormulaView formula={formula} variables={variables} />
        <p class="lens-formula-value-line">ROE = <strong>12.5%</strong></p>
        <FormulaVariables variables={variables} />
      </Card>
      <Card label="variable hover / focus">
        <div ref={(el) => { hoverCell = el }}>
          <FormulaView formula={formula} variables={variables} />
        </div>
      </Card>
      <Card label="loading">
        <FormulaView formula={{ ...formula, markup: undefined }} holdLoading variables={variables} />
      </Card>
      <Card label="invalid LaTeX">
        <FormulaView formula={{ ...formula, latex: String.raw`\frac{a}`, markup: undefined }} variables={variables} />
      </Card>
      <Card label="affordance on a measure with a formula">
        <span>ROE <PanelFormula frame={undefined} panel={panel(true)} /></span>
      </Card>
      <Card label="no formula, no ƒ">
        <span>ROE <PanelFormula frame={undefined} panel={panel(false)} /></span>
      </Card>
    </div>
  )
}

const readerVariables: FormulaVariableView[] = [
  { key: 'N', label: 'Net profit', source: 'General ledger, account 9910', value: '4.2 bn UZS', exact: '4,213,507,880 UZS' },
  { key: 'E', label: 'Average equity', source: 'Balance sheet, section III', availability: 'config_required' },
]

/**
 * What a reader is told around the figures: the panel's note (the ⓘ it
 * replaced), the hint that a click lists the variables, and the pinned list
 * with an operand in full and one whose reason replaces «no data».
 */
function ReaderAids(props: { theme: 'light' | 'dark' }): JSX.Element {
  return (
    <div class="lens-root lens-story-matrix" data-theme={props.theme} style={{ display: 'grid', gap: '16px', 'grid-template-columns': 'repeat(2, minmax(0, 1fr))' }}>
      <Card label="preview: note and hint">
        <strong class="lens-formula-title">How it is calculated</strong>
        <FormulaView formula={formula} variables={readerVariables} />
        <p class="lens-formula-value-line">ROE = <strong>12.5%</strong></p>
        <p class="lens-formula-note">Net of reinsurance; equity is the average of the opening and closing balance.</p>
        <p class="lens-formula-hint">Click ƒ to see the variables and their sources</p>
      </Card>
      <Card label="pinned: exact operand, reason for a gap">
        <FormulaView formula={formula} variables={readerVariables} />
        <FormulaVariables variables={readerVariables} />
      </Card>
    </div>
  )
}

export const ReaderAidsLight = () => <ReaderAids theme="light" />
ReaderAidsLight.storyName = 'Formula reader aids - light'
export const ReaderAidsDark = () => <ReaderAids theme="dark" />
ReaderAidsDark.storyName = 'Formula reader aids - dark'

export const Light = () => <Matrix theme="light" />
Light.storyName = 'Formula states - light'
export const Dark = () => <Matrix theme="dark" />
Dark.storyName = 'Formula states - dark'
