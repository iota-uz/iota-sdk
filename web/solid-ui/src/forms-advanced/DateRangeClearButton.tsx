import { Show, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'
import { callHandler } from '../internal/events'

export interface DateRangeClearButtonProps extends Omit<JSX.ButtonHTMLAttributes<HTMLButtonElement>, 'value'> {
  value?: string | readonly unknown[] | null
  formId?: string
  onClear?: () => void
}

function hasValue(value: DateRangeClearButtonProps['value']): boolean {
  return Array.isArray(value) ? value.length > 0 : String(value ?? '').trim() !== ''
}

export function DateRangeClearButton(props: DateRangeClearButtonProps) {
  const [local, native] = splitProps(props, ['class', 'value', 'formId', 'onClear', 'onClick', 'children', 'aria-label'])
  const clear: JSX.EventHandler<HTMLButtonElement, MouseEvent> = (event) => {
    local.onClear?.()
    if (local.formId) document.getElementById(local.formId)?.dispatchEvent(new CustomEvent('dateRangeChange', { bubbles: true }))
    callHandler(local.onClick, event)
  }
  return (
    <Show when={hasValue(local.value)}>
      <button
        {...native}
        type="button"
        class={classes('absolute right-2 top-1/2 -translate-y-1/2 w-4.5 h-4.5 flex items-center justify-center rounded-full text-gray-400 hover:text-white hover:bg-gray-400 transition-all duration-150 cursor-pointer', local.class)}
        data-form-id={local.formId}
        aria-label={local['aria-label'] ?? 'Clear date range'}
        onClick={clear}
      >
        {local.children ?? (
          <svg aria-hidden="true" width="10" height="10" viewBox="0 0 256 256" xmlns="http://www.w3.org/2000/svg">
            <line x1="200" y1="56" x2="56" y2="200" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" />
            <line x1="200" y1="200" x2="56" y2="56" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" />
          </svg>
        )}
      </button>
    </Show>
  )
}
