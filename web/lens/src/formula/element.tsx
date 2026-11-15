import { render } from 'solid-js/web'
import { z } from 'zod'
import { FormulaTip } from './FormulaTip'
import { FormulaVariables, FormulaView } from './FormulaView'
import type { FormulaVariableView } from './values'
import type { Formula } from '../contract'

const tagName = 'lens-formula'

const formulaSchema = z.object({
  latex: z.string().min(1),
  alt: z.string().optional(),
  markup: z.string().optional(),
  version: z.string().optional(),
  date: z.string().optional(),
  error: z.boolean().optional(),
  variables: z.array(z.object({
    key: z.string(),
    label: z.string(),
    source: z.string().optional(),
    value: z.string().optional(),
    ref: z.object({ kind: z.enum(['measure', 'field', 'stage']), name: z.string() }).optional(),
  })).optional(),
})

/**
 * `<lens-formula>` — the methodology formula outside a Lens dashboard.
 *
 *   <lens-formula formula='{"latex":"…","variables":[{"key":"N","label":"…","value":"1 200"}]}'
 *                 value="12,4 %" label="ROE" mode="block"></lens-formula>
 *
 * The page renders no frame, so the server hands over each variable's
 * formatted value in `variables[].value`. `mode="tip"` (default) renders the ƒ
 * affordance; `mode="block"` renders the formula and its variable list inline.
 * Any children are kept as the no-JS fallback until the element mounts.
 */
export class LensFormulaElement extends HTMLElement {
  static readonly observedAttributes = ['formula', 'value', 'label', 'mode']
  private dispose?: () => void

  connectedCallback() {
    this.draw()
  }

  disconnectedCallback() {
    this.dispose?.()
    this.dispose = undefined
  }

  attributeChangedCallback() {
    if (this.isConnected) this.draw()
  }

  private parse(): Formula | undefined {
    const raw = this.getAttribute('formula')
    if (!raw) return undefined
    try {
      return formulaSchema.parse(JSON.parse(raw)) as Formula
    } catch (cause) {
      console.error('[lens] <lens-formula> has an invalid formula attribute', cause)
      return undefined
    }
  }

  private draw() {
    this.dispose?.()
    const formula = this.parse()
    if (!formula) return
    const variables: FormulaVariableView[] = (formula.variables ?? []).map((v) => ({
      key: v.key, label: v.label, source: v.source, value: v.value,
    }))
    const subject = this.getAttribute('label') ?? ''
    const value = this.getAttribute('value') ?? undefined
    const block = this.getAttribute('mode') === 'block'
    // Solid appends to existing children: drop the server-rendered no-JS
    // fallback now that the component replaces it.
    this.replaceChildren()
    this.dispose = render(() => block
      ? (
        <div class="lens-formula-block">
          <FormulaView formula={formula} variables={variables} />
          {value ? <p class="lens-formula-value-line"><span>{subject}</span> = <strong>{value}</strong></p> : null}
          <FormulaVariables variables={variables} />
        </div>
      )
      : <FormulaTip formula={formula} subject={subject} value={value} variables={variables} />, this)
  }
}

export function registerLensFormulaElement() {
  if (!customElements.get(tagName)) customElements.define(tagName, LensFormulaElement)
}
