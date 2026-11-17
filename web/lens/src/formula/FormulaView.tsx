import { createEffect, createSignal, createUniqueId, For, onCleanup, Show, type JSX } from 'solid-js'
import { renderMath } from '@iota-uz/sdk-math-internal'
import type { Formula } from '../contract'
import { useTranslate } from '../runtime'
import type { FormulaVariableView } from './values'

export interface FormulaViewProps {
  formula: Formula
  /** Variables with their values in this calculation. Without them the symbols are plain. */
  variables?: ReadonlyArray<FormulaVariableView>
  /** Called once when the formula cannot be shown; the report keeps working. */
  onError?: (error: unknown) => void
  /** Holds the view in its loading state; for stories that show the wait. */
  holdLoading?: boolean
}

type RenderState = 'loading' | 'ready' | 'error'

interface Hover {
  key: string
  left: number
  top: number
}

/**
 * Reports a formula that could not be typeset. It is an event, not an
 * exception: the panel shows a message and the rest of the report is untouched.
 */
export function reportFormulaError(error: unknown, formula: Formula): void {
  console.error('[lens] formula could not be rendered', { error, version: formula.version })
}

/**
 * A methodology formula in typeset form.
 *
 * KaTeX is fetched on first use as its own chunk; until it arrives the server's
 * MathML stands in, so there is never an empty hole and a page without script
 * still reads. The LaTeX source is never printed or offered for copying — the
 * reader gets the rendered formula and its text alternative.
 *
 * A symbol the author tagged with `\htmlData{var=KEY}{…}` becomes focusable: hover
 * or Tab shows what that variable is and its value in this calculation.
 */
export function FormulaView(props: FormulaViewProps): JSX.Element {
  const translate = useTranslate()
  const [state, setState] = createSignal<RenderState>(props.formula.error ? 'error' : 'loading')
  const [hover, setHover] = createSignal<Hover>()
  const tipId = createUniqueId()
  let host: HTMLDivElement | undefined

  const alt = () => props.formula.alt?.trim() || ''
  const variable = (key: string) => props.variables?.find((candidate) => candidate.key === key)
  const valueText = (entry: FormulaVariableView | undefined) => entry?.value ?? missingText(translate, entry)

  createEffect(() => {
    const formula = props.formula
    if (formula.error) {
      setState('error')
      reportFormulaError(new Error('invalid formula markup'), formula)
      props.onError?.(new Error('invalid formula markup'))
      return
    }
    setState('loading')
    if (props.holdLoading) return
    let cancelled = false
    onCleanup(() => { cancelled = true })
    renderMath(formula.latex, { displayMode: true, interactive: true }).then(({ html }) => {
      if (cancelled || !host) return
      host.innerHTML = html
      setState('ready')
    }, (error: unknown) => {
      if (cancelled) return
      setState('error')
      reportFormulaError(error, formula)
      props.onError?.(error)
    })
  })

  // Wire the tagged symbols once KaTeX has painted them, and again when the
  // variables (and so their values) change under the same markup.
  createEffect(() => {
    if (state() !== 'ready' || !host) return
    const variables = props.variables ?? []
    const known = new Set(variables.map((entry) => entry.key))
    const cleanups: Array<() => void> = []
    host.querySelectorAll<HTMLElement>('[data-var]').forEach((element) => {
      const key = element.dataset.var ?? ''
      if (!known.has(key)) return
      const entry = variable(key)
      element.tabIndex = 0
      element.classList.add('lens-formula-var')
      element.setAttribute('role', 'button')
      element.setAttribute('aria-label', `${entry?.label ?? key}: ${valueText(entry)}`)
      element.setAttribute('aria-describedby', tipId)
      const show = () => {
        const rect = element.getBoundingClientRect()
        setHover({ key, left: rect.left + rect.width / 2, top: rect.bottom })
      }
      const hide = () => setHover((current) => current?.key === key ? undefined : current)
      const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape') hide() }
      element.addEventListener('mouseenter', show)
      element.addEventListener('focus', show)
      element.addEventListener('mouseleave', hide)
      element.addEventListener('blur', hide)
      element.addEventListener('keydown', onKey)
      cleanups.push(() => {
        element.removeEventListener('mouseenter', show)
        element.removeEventListener('focus', show)
        element.removeEventListener('mouseleave', hide)
        element.removeEventListener('blur', hide)
        element.removeEventListener('keydown', onKey)
      })
    })
    onCleanup(() => cleanups.forEach((cleanup) => cleanup()))
  })

  const active = () => {
    const current = hover()
    return current ? { ...current, entry: variable(current.key) } : undefined
  }

  return (
    <div class="lens-formula" data-state={state()}>
      <Show when={state() === 'error'}>
        <p class="lens-formula-error" role="alert">
          {translate('panel.formula.error', 'The formula could not be displayed.')}
        </p>
      </Show>
      {/* The typeset formula. The wrapper carries the text alternative; KaTeX's
          own hidden MathML twin is not announced a second time. */}
      <div
        aria-label={alt() || undefined}
        class="lens-formula-math"
        hidden={state() === 'error'}
        role="math"
      >
        {/* KaTeX owns this node's children; Solid never renders into it. */}
        <div class="lens-formula-typeset" ref={(el) => { host = el }} />
        {/* Fallback while KaTeX loads: the server-rendered MathML. */}
        <Show when={state() === 'loading'}>
          <Show
            fallback={<span aria-hidden="true" class="lens-shimmer lens-formula-shimmer" />}
            when={props.formula.markup}
          >
            <div class="lens-formula-fallback" innerHTML={props.formula.markup} />
          </Show>
        </Show>
      </div>
      <Show when={active()}>
        {(current) => (
          <div
            class="lens-formula-tip"
            id={tipId}
            role="tooltip"
            style={{ left: `${current().left}px`, top: `${current().top + 6}px` }}
          >
            <span class="lens-formula-tip-label">{current().entry?.label ?? current().key}</span>
            <span class="lens-formula-tip-value" data-missing={current().entry?.value === undefined || undefined}>{valueText(current().entry)}</span>
            <Show when={current().entry?.source}>
              <span class="lens-formula-tip-source">{current().entry?.source}</span>
            </Show>
          </div>
        )}
      </Show>
    </div>
  )
}

/**
 * What a variable without a value says: the reason when the data gives one
 * («Требуется настройка»), otherwise a plain «no data».
 */
function missingText(translate: ReturnType<typeof useTranslate>, entry: FormulaVariableView | undefined): string {
  switch (entry?.availability) {
    case 'config_required': return translate('availability.config_required', 'Configuration required')
    case 'empty_source': return translate('availability.empty_source', 'No source data')
    case 'unavailable': return translate('availability.unavailable', 'Unavailable')
    default: return translate('panel.formula.noValue', 'no data')
  }
}

/** The variable list of the «How it is calculated» panel. */
export function FormulaVariables(props: { variables: ReadonlyArray<FormulaVariableView> }): JSX.Element {
  const translate = useTranslate()
  return (
    <Show when={props.variables.length > 0}>
      <section class="lens-formula-variables">
        <h4 class="lens-formula-subtitle">{translate('panel.formula.variables', 'Variables')}</h4>
        <dl class="lens-formula-variable-list">
          <For each={props.variables}>
            {(entry) => (
              <div class="lens-formula-variable">
                <dt>{entry.label}</dt>
                <dd class="lens-formula-variable-value" data-missing={entry.value === undefined || undefined}>
                  {entry.exact ?? entry.value ?? missingText(translate, entry)}
                </dd>
                <Show when={entry.source}>
                  <dd class="lens-formula-variable-source">
                    {translate('panel.formula.source', 'Source')}: {entry.source}
                  </dd>
                </Show>
              </div>
            )}
          </For>
        </dl>
      </section>
    </Show>
  )
}
