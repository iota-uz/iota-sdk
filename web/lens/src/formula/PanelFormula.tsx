import { createMemo, Show } from 'solid-js'
import type { Formula, Frame, Panel } from '../contract'
import { useLocale } from '../runtime'
import { FormulaTip } from './FormulaTip'
import { formulaValueLine, resolveFormulaVariables } from './values'

interface PanelFormulaProps {
  panel: Panel
  frame: Frame | undefined
  subject?: string
  inline?: boolean
  row?: number
}

function Resolved(props: PanelFormulaProps & { formula: Formula }) {
  const locale = useLocale()
  const variables = createMemo(() => resolveFormulaVariables(props.formula, props.panel, props.frame, locale(), props.row ?? 0))
  const value = createMemo(() => formulaValueLine(props.panel, props.frame, locale(), props.row ?? 0))
  return (
    <FormulaTip
      formula={props.formula}
      inline={props.inline}
      subject={props.subject ?? props.panel.title}
      value={value()}
      variables={variables()}
    />
  )
}

/**
 * The ƒ of a panel that carries a methodology. Values come from the frame the
 * panel already loaded, so no extra request is made and the numbers are those
 * of the filters, period and row on screen. Panels without a formula render
 * nothing — no ƒ, no empty slot — and touch no runtime context.
 */
export function PanelFormula(props: PanelFormulaProps) {
  return (
    <Show when={props.panel.formula}>
      {(formula) => <Resolved {...props} formula={formula()} />}
    </Show>
  )
}
