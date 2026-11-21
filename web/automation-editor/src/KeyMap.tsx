// The "?" map of keys; on touch it starts with gestures (ED5).
import { Dialog } from '@iota-uz/sdk/solid-ui'
import { For, Show, type JSX } from 'solid-js'
import { isCoarse } from './layer'
import { isMac } from './locale'
import { keyLabel, strings } from './strings'
import type { EditorLocale } from './types'

export function KeyMap(props: { open: boolean; locale: EditorLocale; onClose(): void }): JSX.Element {
  const t = () => strings(props.locale)
  const mac = isMac()
  const coarse = isCoarse()
  const table = (title: string, rows: [string, string][], keys: boolean) => (
    <section class="ae-keymap__section">
      <h3 class="ae-keymap__h3">{title}</h3>
      <dl class="ae-keymap__rows">
        <For each={rows}>
          {([key, action]) => (
            <div class="ae-keymap__row">
              <dt>{keys ? <kbd class="ae-kbd">{keyLabel(key, mac)}</kbd> : key}</dt>
              <dd>{action}</dd>
            </div>
          )}
        </For>
      </dl>
    </section>
  )
  return (
    <Dialog open={props.open} onOpenChange={(open) => { if (!open) props.onClose() }} class="ae-dialog" aria-labelledby="ae-keymap-title">
      <div class="ae-dialog__body">
        <h2 id="ae-keymap-title" class="ae-dialog__title" data-dialog-title tabindex="-1">{coarse ? t().keymap.gestures : t().keymap.title}</h2>
        <Show when={coarse} fallback={<>{table(t().keymap.keys, t().keymap.rows, true)}{table(t().keymap.gestures, t().keymap.gestureRows, false)}</>}>
          {table(t().keymap.gestures, t().keymap.gestureRows, false)}
          {table(t().keymap.keys, t().keymap.rows, true)}
        </Show>
        <div class="ae-dialog__actions">
          <button type="button" class="btn btn-secondary btn-normal" onClick={() => props.onClose()}>{t().keymap.close}</button>
        </div>
      </div>
    </Dialog>
  )
}
