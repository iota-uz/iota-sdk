// Graph settings: name, description and run-as with an explanation of each choice.
import { Dialog, Input, Textarea } from '@iota-uz/sdk/solid-ui'
import { For, type JSX } from 'solid-js'
import { strings } from './strings'
import type { AutomationGraph, EditorLocale } from './types'

export interface SettingsDialogProps {
  open: boolean
  graph: AutomationGraph
  locale: EditorLocale
  readOnly: boolean
  /** Host content below the choices: the graph's memory between runs. */
  slot?: JSX.Element
  onChange(patch: Partial<Pick<AutomationGraph, 'name' | 'description' | 'run_as'>>, key: string): void
  onClose(): void
}

export function SettingsDialog(props: SettingsDialogProps): JSX.Element {
  const t = () => strings(props.locale)
  return (
    <Dialog open={props.open} onOpenChange={(open) => { if (!open) props.onClose() }} class={props.slot ? 'ae-dialog ae-dialog--wide' : 'ae-dialog'} aria-labelledby="ae-settings-title">
      <div class="ae-dialog__body">
        <h2 id="ae-settings-title" class="ae-dialog__title" data-dialog-title tabindex="-1">{t().settings.title}</h2>
        <Input
          label={t().settings.name}
          value={props.graph.name}
          readOnly={props.readOnly}
          maxLength={200}
          onInput={(event) => props.onChange({ name: event.currentTarget.value }, 'settings:name')}
        />
        <Textarea
          label={t().settings.description}
          value={props.graph.description ?? ''}
          readOnly={props.readOnly}
          rows={3}
          onInput={(event) => props.onChange({ description: event.currentTarget.value || undefined }, 'settings:description')}
        />
        <fieldset class="ae-radio-group" disabled={props.readOnly}>
          <legend class="ae-radio-group__legend">{t().settings.runAs}</legend>
          <For each={['owner', 'initiator'] as const}>
            {(value) => (
              <label class="ae-radio-card" classList={{ 'ae-radio-card--checked': props.graph.run_as === value }}>
                <input
                  type="radio"
                  name="ae-run-as"
                  value={value}
                  checked={props.graph.run_as === value}
                  onChange={() => props.onChange({ run_as: value }, 'settings:run_as')}
                />
                <span class="ae-radio-card__text">
                  <span class="ae-radio-card__title">{value === 'owner' ? t().settings.owner : t().settings.initiator}</span>
                  <span class="ae-radio-card__hint">{value === 'owner' ? t().settings.ownerHint : t().settings.initiatorHint}</span>
                </span>
              </label>
            )}
          </For>
        </fieldset>
        {props.slot}
        <div class="ae-dialog__actions">
          <button type="button" class="btn btn-primary btn-normal" onClick={() => props.onClose()}>{t().settings.done}</button>
        </div>
      </div>
    </Dialog>
  )
}
