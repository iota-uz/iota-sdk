import { createEffect, createSignal, createUniqueId, onCleanup, Show, untrack } from 'solid-js'
import { Portal } from 'solid-js/web'
import type { Formula } from '../contract'
import { FormulaMark } from '../icons'
import { useTranslate } from '../runtime'
import { hoverBridgeDelay, useOverlayContainer } from '../panels/overlayContainer'
import { infoTipTailOffset, positionInfoTip } from '../panels/InfoTip'
import { FormulaVariables, FormulaView } from './FormulaView'
import type { FormulaVariableView } from './values'

export interface FormulaTipProps {
  formula: Formula
  /** The figure's name; names the trigger for a reader who cannot see what it sits beside. */
  subject: string
  /** The figure in this calculation, formatted — shown under the formula. */
  value?: string
  variables?: ReadonlyArray<FormulaVariableView>
  /** Compact form for a metric strip cell (see InfoTip). */
  inline?: boolean
}

interface Placement {
  left: number
  top: number
  side: 'above' | 'below'
}

/**
 * The ƒ in a figure's header. Hover or focus previews the formula with the
 * figure's value under it; a click pins the full «How it is calculated» panel
 * with every variable, its value in this slice, its source and the methodology
 * version. Same interaction model as ⓘ, so one muscle memory serves both.
 */
export function FormulaTip(props: FormulaTipProps) {
  const translate = useTranslate()
  const [pinned, setPinned] = createSignal(false)
  const [hovered, setHovered] = createSignal(false)
  const [placement, setPlacement] = createSignal<Placement>()
  const [tail, setTail] = createSignal(14)
  const [bubbleRef, setBubbleRef] = createSignal<HTMLElement>()
  let wrapper: HTMLSpanElement | undefined
  let button: HTMLButtonElement | undefined
  let closeTimer: ReturnType<typeof globalThis.setTimeout> | undefined
  const bubbleId = createUniqueId()
  const open = () => pinned() || hovered()
  const container = useOverlayContainer(open, () => wrapper, 'lens-info-tip-overlay-root')

  const reposition = () => {
    const anchor = button?.getBoundingClientRect()
    const bubble = bubbleRef()?.getBoundingClientRect()
    if (!anchor || !bubble) return
    const next = positionInfoTip(anchor, bubble, { width: globalThis.innerWidth || 1024, height: globalThis.innerHeight || 768 })
    setPlacement(next)
    setTail(infoTipTailOffset((anchor.left + anchor.right) / 2, next.left, bubble.width))
  }

  createEffect(() => { if (!open()) setPlacement(undefined) })
  createEffect(() => {
    if (container() && bubbleRef()) untrack(reposition)
  })
  // The bubble grows when KaTeX replaces the fallback and when the variable
  // list opens; keep it on screen as it does.
  createEffect(() => {
    if (!container()) return
    const observer = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(reposition)
    const bubble = bubbleRef()
    if (bubble) observer?.observe(bubble)
    globalThis.addEventListener('resize', reposition)
    globalThis.addEventListener('scroll', reposition, true)
    onCleanup(() => {
      observer?.disconnect()
      globalThis.removeEventListener('resize', reposition)
      globalThis.removeEventListener('scroll', reposition, true)
    })
  })
  createEffect(() => {
    if (!open()) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        // Focus first: the trigger's onFocus opens the preview, and the reset
        // below must be the last word.
        button?.focus()
        setPinned(false)
        setHovered(false)
      }
    }
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target as Node
      if (!wrapper?.contains(target) && !bubbleRef()?.contains(target)) setPinned(false)
    }
    document.addEventListener('keydown', onKeyDown)
    document.addEventListener('pointerdown', onPointerDown)
    onCleanup(() => {
      document.removeEventListener('keydown', onKeyDown)
      document.removeEventListener('pointerdown', onPointerDown)
    })
  })

  const cancelClose = () => {
    if (closeTimer !== undefined) globalThis.clearTimeout(closeTimer)
    closeTimer = undefined
  }
  const leaving = (event: MouseEvent, other: HTMLElement | null | undefined) => {
    const next = event.relatedTarget
    if (next instanceof Node && other?.contains(next)) return
    cancelClose()
    closeTimer = globalThis.setTimeout(() => setHovered(false), hoverBridgeDelay)
  }
  onCleanup(cancelClose)

  const label = () => translate('panel.formula.show', 'Show how {name} is calculated', { name: props.subject })
  const version = () => {
    const { version: v, date } = props.formula
    if (!v && !date) return ''
    return translate('panel.formula.version', 'Methodology v{version} · {date}', { version: v ?? '', date: date ?? '' })
      .replace(/\s*·\s*$/, '')
  }

  return (
    <span
      class={props.inline ? 'lens-info-tip lens-info-tip-inline lens-formula-tip-wrap' : 'lens-info-tip lens-formula-tip-wrap'}
      onMouseEnter={() => { cancelClose(); setHovered(true) }}
      onMouseLeave={(event) => leaving(event, bubbleRef())}
      ref={(el) => { wrapper = el }}
    >
      <button
        aria-controls={open() ? bubbleId : undefined}
        aria-expanded={open()}
        aria-label={label()}
        class={props.inline
          ? `lens-export-button lens-icon-button lens-info-tip-button lens-info-tip-button-inline lens-formula-button${open() ? ' lens-info-tip-button-open' : ''}`
          : `lens-export-button lens-icon-button lens-info-tip-button lens-formula-button${open() ? ' lens-info-tip-button-open' : ''}`}
        data-testid="lens-formula-button"
        onBlur={() => setHovered(false)}
        onClick={() => setPinned((current) => !current)}
        onFocus={() => setHovered(true)}
        ref={(el) => { button = el }}
        type="button"
      >
        <FormulaMark />
      </button>
      <Show when={open() && container()}>
        <Portal mount={container()}>
          <section
            aria-label={translate('panel.formula', 'How it is calculated')}
            class="lens-info-tip-bubble lens-formula-bubble"
            data-detailed={pinned() || undefined}
            data-side={placement()?.side ?? 'below'}
            id={bubbleId}
            onMouseEnter={() => { cancelClose(); setHovered(true) }}
            onMouseLeave={(event) => leaving(event, wrapper)}
            ref={setBubbleRef}
            style={{
              left: `${placement()?.left ?? 0}px`,
              top: `${placement()?.top ?? 0}px`,
              'pointer-events': 'auto',
              visibility: placement() ? 'visible' : 'hidden',
              '--lens-info-tip-tail': `${tail()}px`,
            }}
          >
            <span aria-hidden="true" class="lens-info-tip-tail" />
            <span class="lens-info-tip-body lens-formula-body">
              <strong class="lens-formula-title">{translate('panel.formula', 'How it is calculated')}</strong>
              <FormulaView formula={props.formula} variables={props.variables} />
              <Show when={props.value}>
                <p class="lens-formula-value-line" data-testid="lens-formula-value">
                  <span>{props.subject}</span> = <strong>{props.value}</strong>
                </p>
              </Show>
              <Show when={pinned()}>
                <FormulaVariables variables={props.variables ?? []} />
                <Show when={version()}><p class="lens-formula-version">{version()}</p></Show>
              </Show>
            </span>
          </section>
        </Portal>
      </Show>
    </span>
  )
}
