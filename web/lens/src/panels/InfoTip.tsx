import { useEffect, useId, useRef, useState } from 'react'
import { useTranslate } from '../runtime'
import { Info } from '../icons'

export interface InfoTipProps {
  /** Already-localized note. Newlines separate paragraphs. */
  text: string
  /**
   * Compact form for a metric strip cell, whose label is 10px type: the
   * header glyph's 28px hit box would tower over it. Also lifts the control
   * above a card-wide navigate anchor, which otherwise swallows the click.
   */
  inline?: boolean
}

/**
 * The ⓘ affordance in a panel's header.
 *
 * A panel's supporting note explains how a figure is obtained and what it
 * excludes — a paragraph most readers need once. Rendered as chrome it costs
 * a permanent band above every card and pushes the plot itself below the
 * fold; behind this glyph it costs nothing until asked for. Print keeps the
 * same text as the figure's footnote, where there is nothing to ask.
 *
 * Hover and focus open it; a click pins it open so the text can be read on a
 * touch screen and selected with the pointer.
 */
export function InfoTip({ text, inline }: InfoTipProps) {
  const translate = useTranslate()
  const [pinned, setPinned] = useState(false)
  const [hovered, setHovered] = useState(false)
  const wrapperRef = useRef<HTMLSpanElement>(null)
  const bubbleId = useId()
  const label = translate('panel.info', 'About this metric')
  const open = pinned || hovered

  // A pinned bubble is dismissed the way every other transient surface in the
  // runtime is: Escape, or a click that lands anywhere else.
  useEffect(() => {
    if (!pinned) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setPinned(false)
    }
    const onPointerDown = (event: PointerEvent) => {
      if (!wrapperRef.current?.contains(event.target as Node)) setPinned(false)
    }
    document.addEventListener('keydown', onKeyDown)
    document.addEventListener('pointerdown', onPointerDown)
    return () => {
      document.removeEventListener('keydown', onKeyDown)
      document.removeEventListener('pointerdown', onPointerDown)
    }
  }, [pinned])

  const paragraphs = text.split(/\n{2,}/).map((paragraph) => paragraph.trim()).filter(Boolean)

  return (
    <span
      // Class names stay literal: Tailwind's content scan cannot see an
      // interpolated modifier and would drop the rule.
      className={inline ? 'lens-info-tip lens-info-tip-inline' : 'lens-info-tip'}
      ref={wrapperRef}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      <button
        aria-describedby={open ? bubbleId : undefined}
        aria-expanded={pinned}
        aria-label={label}
        className={inline
          ? 'lens-export-button lens-icon-button lens-info-tip-button lens-info-tip-button-inline'
          : 'lens-export-button lens-icon-button lens-info-tip-button'}
        onBlur={() => setHovered(false)}
        onClick={() => setPinned((current) => !current)}
        onFocus={() => setHovered(true)}
        title={label}
        type="button"
      >
        <Info />
      </button>
      {open && (
        <span className="lens-info-tip-bubble" id={bubbleId} role="tooltip">
          <span className="lens-info-tip-title">{label}</span>
          {paragraphs.map((paragraph, index) => (
            <span className="lens-info-tip-text" key={index}>{paragraph}</span>
          ))}
        </span>
      )}
    </span>
  )
}
