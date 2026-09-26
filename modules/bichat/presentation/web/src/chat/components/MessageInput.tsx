import { Show, createSignal, type JSX } from 'solid-js'
import type { Accessor } from 'solid-js'
import { useI18n } from '../../i18n/i18n'

const MAX_ROWS_HEIGHT_PX = 150

export function MessageInput(props: {
  disabled?: boolean
  streaming?: boolean | Accessor<boolean>
  onSend: (content: string) => void
  onStop: () => void
}): JSX.Element {
  const i18n = useI18n()
  const [value, setValue] = createSignal('')
  let textarea: HTMLTextAreaElement | undefined

  const isStreaming = (): boolean =>
    typeof props.streaming === 'function' ? (props.streaming as Accessor<boolean>)() : props.streaming === true

  const blocked = (): boolean => props.disabled === true || isStreaming()

  const autosize = (): void => {
    if (!textarea) return
    textarea.style.height = 'auto'
    textarea.style.height = `${Math.min(textarea.scrollHeight, MAX_ROWS_HEIGHT_PX)}px`
    textarea.style.overflowY = textarea.scrollHeight > MAX_ROWS_HEIGHT_PX ? 'auto' : 'hidden'
  }

  const send = (): void => {
    const content = value().trim()
    if (!content || blocked()) return
    props.onSend(content)
    setValue('')
    queueMicrotask(autosize)
  }

  const handleKeyDown = (event: KeyboardEvent): void => {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault()
      send()
      return
    }
    if (event.key === 'Escape') {
      setValue('')
      queueMicrotask(autosize)
    }
  }

  return (
    <form
      class="shrink-0 border-t border-neutral-200 bg-white px-4 py-3"
      onSubmit={(event) => {
        event.preventDefault()
        send()
      }}
    >
      <div class="flex items-end gap-2">
        <textarea
          ref={textarea}
          rows={1}
          class="max-h-[150px] flex-1 resize-none rounded-xl border border-neutral-300 px-3 py-2 text-sm text-neutral-900 placeholder:text-neutral-400 focus:border-sky-500 focus:outline-none focus:ring-1 focus:ring-sky-500 disabled:bg-neutral-100 disabled:text-neutral-400"
          placeholder={i18n.t('chat.placeholder')}
          disabled={props.disabled === true}
          value={value()}
          onInput={(event) => {
            setValue(event.currentTarget.value)
            autosize()
          }}
          onKeyDown={handleKeyDown}
        />
        <Show
          when={isStreaming()}
          fallback={
            <button
              type="submit"
              aria-label={i18n.t('chat.send')}
              class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-sky-600 text-white transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:bg-neutral-300"
              disabled={blocked() || value().trim().length === 0}
            >
              <svg viewBox="0 0 24 24" fill="currentColor" class="h-4 w-4" aria-hidden="true">
                <path d="M3.478 2.404a.75.75 0 0 0-.926.941l2.432 7.905H13.5a.75.75 0 0 1 0 1.5H4.984l-2.432 7.905a.75.75 0 0 0 .926.94 60.519 60.519 0 0 0 18.445-8.986.75.75 0 0 0 0-1.218A60.517 60.517 0 0 0 3.478 2.404Z" />
              </svg>
            </button>
          }
        >
          <button
            type="button"
            aria-label={i18n.t('chat.stop')}
            class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-neutral-800 text-white transition hover:bg-neutral-700"
            onClick={() => props.onStop()}
          >
            <svg viewBox="0 0 20 20" fill="currentColor" class="h-3.5 w-3.5" aria-hidden="true">
              <rect x="4" y="4" width="12" height="12" rx="2" />
            </svg>
          </button>
        </Show>
      </div>
    </form>
  )
}
