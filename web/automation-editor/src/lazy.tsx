// The bounded entry keeps the canvas, @xyflow/system and d3 in a lazy chunk: importing the subpath costs
// the pure helpers and these wrappers; the chunk loads when an Editor or Canvas first renders.
import { lazy, Suspense, type JSX } from 'solid-js'
import { CARD_HEIGHT } from './geometry'
import { strings } from './strings'
import type { CanvasProps, EditorLocale, EditorProps } from './types'

type Chunk = typeof import('./chunk')
let chunk: Promise<Chunk> | null = null

function chunkPromise(): Promise<Chunk> {
  chunk ??= import('./chunk')
  return chunk
}

/** Starts loading the editor chunk (e.g. on hover of a link to the editor). */
export function loadEditor(): Promise<void> {
  return chunkPromise().then(() => undefined)
}

const LazyEditor = lazy(() => chunkPromise().then((m) => ({ default: m.Editor })))
const LazyCanvas = lazy(() => chunkPromise().then((m) => ({ default: m.Canvas })))

/** The loading state: a skeleton of the toolbar and the canvas with the real cards' size (S2). */
export function EditorSkeleton(props: { locale?: EditorLocale }): JSX.Element {
  const cards = [
    { x: 48, y: 64, w: 224 },
    { x: 368, y: 64, w: 240 },
    { x: 368, y: 64 + CARD_HEIGHT + 48, w: 224 },
    { x: 704, y: 64, w: 224 },
  ]
  return (
    <div class="ae-editor ae-skeleton" role="status" aria-busy="true" aria-label={strings(props.locale).states.loading}>
      <div class="ae-toolbar">
        <span class="ae-skeleton__bar" style={{ width: '220px' }} />
        <span class="ae-skeleton__bar" style={{ width: '320px', 'margin-inline-start': 'auto' }} />
      </div>
      <div class="ae-work">
        <div class="ae-main">
          <div class="ae-area">
            <div class="ae-canvas ae-skeleton__canvas">
              {cards.map((c) => <span class="ae-skeleton__card" style={{ left: `${c.x}px`, top: `${c.y}px`, width: `${c.w}px`, height: `${CARD_HEIGHT}px` }} />)}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export function Editor(props: EditorProps): JSX.Element {
  return (
    <Suspense fallback={<EditorSkeleton locale={props.locale} />}>
      <LazyEditor {...props} />
    </Suspense>
  )
}

export function Canvas(props: CanvasProps): JSX.Element {
  return (
    <Suspense fallback={<div class="ae-canvas ae-skeleton__canvas" role="status" aria-busy="true" aria-label={strings(props.locale).states.loading} />}>
      <LazyCanvas {...props} />
    </Suspense>
  )
}
