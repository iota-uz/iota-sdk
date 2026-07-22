/**
 * Phosphor icon glyphs, inlined.
 *
 * The runtime used Unicode characters (⤢, ↓, ↻, →) which differ from the
 * legacy renderer's Phosphor set in shape, weight and baseline, and which
 * change with whatever font the host page loads. These are the same regular
 * (16px stroke on a 256 grid) paths the Go renderer emits through
 * github.com/iota-uz/icons/phosphor, inlined so no request leaves the page.
 *
 * Every glyph is decorative: the interactive element around it carries the
 * accessible name.
 */

import type { ReactNode } from 'react'

export interface IconProps {
  /** Square size in CSS pixels. Defaults match the legacy renderer's usage. */
  size?: number
  className?: string
}

function glyph(children: ReactNode, defaultSize: number) {
  return function Glyph({ size = defaultSize, className }: IconProps) {
    return (
    <svg
        aria-hidden="true"
        className={className ? `lens-icon ${className}` : 'lens-icon'}
        focusable="false"
        height={size}
        viewBox="0 0 256 256"
        width={size}
        xmlns="http://www.w3.org/2000/svg"
      >
        {children}
    </svg>
    )
  }
}

/** Expand a panel to fullscreen. */
export const ArrowsOut = glyph(
  <>
    <polyline points="160 48 208 48 208 96" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
    <line x1="152" y1="104" x2="208" y2="48" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
    <polyline points="96 208 48 208 48 160" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
    <line x1="104" y1="152" x2="48" y2="208" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
    <polyline points="208 160 208 208 160 208" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
    <line x1="152" y1="152" x2="208" y2="208" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
    <polyline points="48 96 48 48 96 48" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
    <line x1="104" y1="104" x2="48" y2="48" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
  </>,
  14,
)

/** Collapse an expanded panel. */
export const ArrowsIn = glyph(
  <>
    <polyline points="192 104 152 104 152 64" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
    <line x1="208" y1="48" x2="152" y2="104" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
    <polyline points="64 152 104 152 104 192" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
    <line x1="48" y1="208" x2="104" y2="152" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
    <polyline points="152 192 152 152 192 152" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
    <line x1="208" y1="208" x2="152" y2="152" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
    <polyline points="104 64 104 104 64 104" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
    <line x1="48" y1="48" x2="104" y2="104" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
  </>,
  14,
)

/** Export / download. */
export const DownloadSimple = glyph(
  <>
    <line x1="128" y1="144" x2="128" y2="32" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
    <polyline points="216 144 216 208 40 208 40 144" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
    <polyline points="168 104 128 144 88 104" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
  </>,
  14,
)

/** Retry a failed export. */
export const ArrowClockwise = glyph(
  <>
    <polyline points="184 104 232 104 232 56" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
    <path d="M188.4,192a88,88,0,1,1,1.83-126.23L232,104" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
  </>,
  14,
)

/** Pending spinner; pair with the lens-icon-spin class. */
export const CircleNotch = glyph(
  <>
    <path d="M168,40a97,97,0,0,1,56,88,96,96,0,0,1-192,0A97,97,0,0,1,88,40" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
  </>,
  14,
)

/** Dismiss an overlay. */
export const X = glyph(
  <>
    <line x1="200" y1="56" x2="56" y2="200" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" fill="currentColor" />
    <line x1="200" y1="200" x2="56" y2="56" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" fill="currentColor" />
  </>,
  12,
)

/** Go back one drill level. */
export const CaretLeft = glyph(
  <>
    <polyline points="160 208 80 128 160 48" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
  </>,
  16,
)

/** Trail separator and breakdown row chevron. */
export const CaretRight = glyph(
  <>
    <polyline points="96 48 176 128 96 208" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
  </>,
  11,
)

/** Explore affordance in a panel header. */
export const CaretDown = glyph(
  <>
    <polyline points="208 96 128 176 48 96" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
  </>,
  14,
)

/** Copy the segment value to the clipboard. */
export const Copy = glyph(
  <>
    <rect x="88" y="88" width="128" height="128" rx="8" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
    <path d="M40,168H32a8,8,0,0,1-8-8V40a8,8,0,0,1,8-8H160a8,8,0,0,1,8,8v8" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
  </>,
  13,
)

/** Confirmation that the value was copied. */
export const Check = glyph(
  <>
    <polyline points="216 72 104 184 48 128" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
  </>,
  13,
)

/** Leaf link: open the underlying records. */
export const ArrowUpRight = glyph(
  <>
    <line x1="64" y1="192" x2="192" y2="64" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
    <polyline points="88 64 192 64 192 168" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="16" />
  </>,
  12,
)
