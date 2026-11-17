/**
 * The one KaTeX seam shared by chat-ui and Lens.
 *
 * KaTeX is ~270 KB; neither product needs it to paint its first frame, so it is
 * only ever reached through `loadKatex()`, a dynamic import that bundlers split
 * into its own chunk. Both products take their options from here so a formula
 * means the same thing in a chat answer and in a report.
 */
import type { KatexOptions } from 'katex'

export type { KatexOptions }

type KatexModule = typeof import('katex').default

let katexPromise: Promise<KatexModule> | undefined

/** Loads KaTeX (and its stylesheet) once, on first use. */
export function loadKatex(): Promise<KatexModule> {
  katexPromise ??= Promise.all([
    import('katex'),
    import('katex/dist/katex.min.css'),
  ]).then(([module]) => module.default, (error: unknown) => {
    // A failed load must not poison every later render: let the next call retry.
    katexPromise = undefined
    throw error
  })
  return katexPromise
}

/**
 * Options every consumer shares. LaTeX that reaches us is authored by the host
 * application (a report methodology) or by a model (a chat answer); neither may
 * run URLs, so `trust` stays off and unknown commands do not abort a render.
 */
export const baseKatexOptions: KatexOptions = {
  strict: 'ignore',
  trust: false,
}

/**
 * Options for formulas whose variables a reader can inspect. The only trusted
 * command is `\htmlData`, which merely attaches `data-*` attributes — it is how
 * a rendered symbol is tied back to its variable.
 */
export const interactiveKatexOptions: KatexOptions = {
  ...baseKatexOptions,
  trust: (context) => context.command === '\\htmlData',
}

export interface RenderedMath {
  html: string
}

/**
 * Renders LaTeX to markup. Rejects with the KaTeX parse error when the source
 * is invalid, so the caller can show a message and report the event instead of
 * letting KaTeX paint the raw source in red.
 */
export async function renderMath(
  latex: string,
  options: { displayMode?: boolean; interactive?: boolean } = {},
): Promise<RenderedMath> {
  const katex = await loadKatex()
  const html = katex.renderToString(latex, {
    ...(options.interactive ? interactiveKatexOptions : baseKatexOptions),
    displayMode: options.displayMode ?? true,
    throwOnError: true,
    output: 'htmlAndMathml',
  })
  return { html }
}
