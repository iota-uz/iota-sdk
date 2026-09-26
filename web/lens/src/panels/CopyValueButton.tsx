/* eslint-disable react/no-unknown-property -- Solid JSX uses `class`, the React-era rule expects `className`; the lint config migrates with the Solid port. */
import { createSignal, onCleanup } from 'solid-js'
import { useTranslate } from '../runtime'
import { copyText } from '../runtime/clipboard'
import { Check, Copy } from '../icons'

export interface CopyValueButtonProps {
  /** The machine value, already in the unit the figure beside it is drawn in. */
  raw: string
}

/**
 * Takes the figure it sits beside, unformatted, to the clipboard.
 *
 * A dashboard figure is read and then used somewhere else — a message, a
 * spreadsheet, a note — and selecting «−41 200 000 000 UZS» by hand yields a
 * string no spreadsheet will accept. The button pastes plain digits instead,
 * and the check it turns into is the only confirmation there is: nothing else
 * on screen changes when a copy succeeds.
 */
export function CopyValueButton(props: CopyValueButtonProps) {
  const translate = useTranslate()
  const [copied, setCopied] = createSignal(false)
  let timer: ReturnType<typeof setTimeout> | undefined
  onCleanup(() => { if (timer) clearTimeout(timer) })
  const label = () => copied() ? translate('explore.copied', 'Copied') : translate('explore.copyValue', 'Copy value')
  return (
    <button
      aria-label={label()}
      class="lens-icon-button lens-tooltip-copy"
      data-copied={copied() ? 'true' : undefined}
      onClick={(event) => {
        // The tooltip floats over a column that is itself a drill target. A
        // copy is not a drill, so the activation stops here.
        event.stopPropagation()
        // Confirmed on the act, not on the promise. An unfocused document can
        // leave `clipboard.writeText` pending indefinitely rather than
        // rejecting, and a button that silently never confirms is worse than
        // one that confirms a write the browser then refused — the value is
        // still on screen either way.
        void copyText(props.raw)
        setCopied(true)
        if (timer) clearTimeout(timer)
        timer = setTimeout(() => setCopied(false), 1500)
      }}
      title={label()}
      type="button"
    >
      {copied() ? <Check /> : <Copy />}
    </button>
  )
}
