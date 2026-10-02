import { For, Show, createEffect, createSignal, type JSX } from 'solid-js'
import type { Accessor } from 'solid-js'
import type { ChatTurn, ToolCallView } from '../types'
import { MarkdownRenderer } from './MarkdownRenderer'
import { useI18n } from '../../i18n/i18n'

const NEAR_BOTTOM_PX = 80

function ToolChip(props: { call: ToolCallView }): JSX.Element {
  const i18n = useI18n()
  const label = () =>
    props.call.status === 'running'
      ? i18n.t('chat.toolRunning', { name: props.call.name })
      : i18n.t('chat.toolFinished', { name: props.call.name })
  return (
    <span
      class="inline-flex max-w-full items-center gap-1.5 rounded-full border border-neutral-200 bg-neutral-50 px-2.5 py-1 text-xs text-neutral-600"
      classList={{ 'border-red-200 bg-red-50 text-red-700': props.call.status === 'failed' }}
      title={props.call.error ?? props.call.detail}
    >
      <Show
        when={props.call.status === 'running'}
        fallback={
          <Show
            when={props.call.status === 'failed'}
            fallback={
              <svg viewBox="0 0 20 20" fill="currentColor" class="h-3 w-3 text-emerald-600" aria-hidden="true">
                <path
                  fill-rule="evenodd"
                  d="M16.704 5.29a1 1 0 0 1 .006 1.415l-7.2 7.3a1 1 0 0 1-1.427.006L3.3 9.16a1 1 0 1 1 1.416-1.412l3.49 3.47 6.494-6.583a1 1 0 0 1 1.414-.006Z"
                  clip-rule="evenodd"
                />
              </svg>
            }
          >
            <svg viewBox="0 0 20 20" fill="currentColor" class="h-3 w-3 text-red-500" aria-hidden="true">
              <path d="M6.28 5.22a.75.75 0 0 0-1.06 1.06L8.94 10l-3.72 3.72a.75.75 0 1 0 1.06 1.06L10 11.06l3.72 3.72a.75.75 0 1 0 1.06-1.06L11.06 10l3.72-3.72a.75.75 0 0 0-1.06-1.06L10 8.94 6.28 5.22Z" />
            </svg>
          </Show>
        }
      >
        <span class="h-2 w-2 shrink-0 animate-pulse rounded-full bg-sky-500" aria-hidden="true" />
      </Show>
      <span class="truncate">{label()}</span>
      <Show when={props.call.status === 'failed' && props.call.error}>
        <span class="max-w-48 truncate text-red-600">{props.call.error}</span>
      </Show>
    </span>
  )
}

function TurnView(props: {
  turn: ChatTurn
  onRetry?: () => void
}): JSX.Element {
  const i18n = useI18n()
  const [thinkingOpen, setThinkingOpen] = createSignal(false)
  const assistant = () => props.turn.assistant

  return (
    <div class="space-y-2">
      <div class="flex justify-end">
        <div class="max-w-[85%] rounded-2xl rounded-br-sm bg-sky-600 px-4 py-2 text-sm text-white">
          <span class="sr-only">{i18n.t('chat.you')}: </span>
          <p class="whitespace-pre-wrap break-words">{props.turn.userContent}</p>
        </div>
      </div>
      <Show when={assistant()}>
        {(asst) => (
          <div class="space-y-2">
            <div class="text-xs font-medium text-neutral-500">{i18n.t('chat.assistant')}</div>
            <Show when={asst().thinking.length > 0}>
              <details
                class="rounded-lg border border-neutral-200 bg-neutral-50 px-3 py-2"
                open={thinkingOpen()}
                onToggle={(event) => setThinkingOpen(event.currentTarget.open)}
              >
                <summary class="cursor-pointer select-none text-xs text-neutral-500">
                  {thinkingOpen() ? i18n.t('chat.hideThinking') : i18n.t('chat.showThinking')}
                </summary>
                <div class="mt-2 space-y-1 text-xs text-neutral-500">
                  <For each={asst().thinking}>{(entry) => <p class="whitespace-pre-wrap">{entry}</p>}</For>
                </div>
              </details>
            </Show>
            <Show when={asst().toolCalls.length > 0}>
              <div class="flex flex-wrap gap-1.5">
                <For each={asst().toolCalls}>{(call) => <ToolChip call={call} />}</For>
              </div>
            </Show>
            <div class="space-y-2">
              <For each={asst().textBlocks}>
                {(block) => (
                  <Show
                    when={block.content.length > 0}
                    fallback={
                      <Show when={props.turn.status === 'streaming'}>
                        <span class="inline-flex items-center gap-1 py-2" aria-label={i18n.t('chat.thinking')}>
                          <span class="h-1.5 w-1.5 animate-pulse rounded-full bg-neutral-400" />
                          <span class="h-1.5 w-1.5 animate-pulse rounded-full bg-neutral-400 [animation-delay:150ms]" />
                          <span class="h-1.5 w-1.5 animate-pulse rounded-full bg-neutral-400 [animation-delay:300ms]" />
                        </span>
                      </Show>
                    }
                  >
                    <MarkdownRenderer content={block.content} />
                  </Show>
                )}
              </For>
            </div>
            <Show when={asst().citations.length > 0}>
              <div class="space-y-1">
                <div class="text-xs font-medium text-neutral-500">{i18n.t('chat.sources')}</div>
                <ol class="space-y-0.5">
                  <For each={asst().citations}>
                    {(citation, index) => (
                      <li class="text-xs">
                        <a
                          href={citation.url}
                          target="_blank"
                          rel="noopener"
                          class="text-sky-700 underline decoration-sky-300 hover:decoration-sky-500"
                        >
                          {index() + 1}. {citation.title}
                        </a>
                        <Show when={citation.source}>
                          <span class="ml-1 text-neutral-400">({citation.source})</span>
                        </Show>
                      </li>
                    )}
                  </For>
                </ol>
              </div>
            </Show>
            <Show when={asst().usage?.totalTokens}>
              {(total) => (
                <div class="text-xs text-neutral-400">
                  {i18n.t('chat.usage', { total: total() })}
                </div>
              )}
            </Show>
          </div>
        )}
      </Show>
      <Show when={props.turn.status === 'failed'}>
        <div class="flex items-center justify-between gap-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
          <span>{i18n.t('chat.error')}</span>
          <Show when={props.onRetry}>
            <button
              type="button"
              class="rounded-md border border-red-300 bg-white px-2.5 py-1 text-xs font-medium text-red-700 transition hover:bg-red-50"
              onClick={() => props.onRetry?.()}
            >
              {i18n.t('chat.retry')}
            </button>
          </Show>
        </div>
      </Show>
      <Show when={props.turn.status === 'cancelled'}>
        <div class="rounded-lg border border-neutral-200 bg-neutral-50 px-3 py-2 text-xs text-neutral-500">
          {i18n.t('chat.cancelled')}
        </div>
      </Show>
    </div>
  )
}

export function MessageList(props: {
  turns: Accessor<ChatTurn[]>
  loading?: Accessor<boolean>
  streaming?: Accessor<boolean>
  error?: Accessor<string | undefined>
  onRetry?: () => void
}): JSX.Element {
  const i18n = useI18n()
  const [showJump, setShowJump] = createSignal(false)
  let container: HTMLDivElement | undefined

  const nearBottom = (): boolean => {
    if (!container) return true
    return container.scrollHeight - container.scrollTop - container.clientHeight < NEAR_BOTTOM_PX
  }

  const scrollToBottom = (): void => {
    if (container) container.scrollTop = container.scrollHeight
  }

  const lastTextLength = (): number => {
    const turns = props.turns()
    const last = turns[turns.length - 1]
    const blocks = last?.assistant?.textBlocks
    if (!blocks || blocks.length === 0) return 0
    return blocks[blocks.length - 1].content.length
  }

  createEffect(() => {
    props.turns()
    lastTextLength()
    if (nearBottom()) scrollToBottom()
  })

  const handleScroll = (): void => {
    setShowJump(!nearBottom())
  }

  const isEmpty = () => props.turns().length === 0

  return (
    <div class="relative min-h-0 flex-1">
      <div
        ref={container}
        class="flex h-full flex-col gap-6 overflow-y-auto px-4 py-4"
        onScroll={handleScroll}
      >
        <Show
          when={!props.loading?.() || !isEmpty()}
          fallback={
            <div class="space-y-4" aria-busy="true">
              <div class="h-4 w-2/3 animate-pulse rounded bg-neutral-200" />
              <div class="h-4 w-1/2 animate-pulse rounded bg-neutral-200" />
              <div class="h-4 w-3/5 animate-pulse rounded bg-neutral-200" />
            </div>
          }
        >
          <Show when={!isEmpty()} fallback={<span class="sr-only">{i18n.t('chat.noMessages')}</span>}>
            <For each={props.turns()}>
              {(turn) => <TurnView turn={turn} onRetry={props.onRetry} />}
            </For>
          </Show>
        </Show>
      </div>
      <Show when={showJump()}>
        <button
          type="button"
          aria-label={i18n.t('chat.jumpToLatest')}
          class="absolute bottom-4 left-1/2 -translate-x-1/2 rounded-full border border-neutral-200 bg-white p-2 text-neutral-600 shadow-md transition hover:bg-neutral-50"
          onClick={() => {
            scrollToBottom()
            setShowJump(false)
          }}
        >
          <svg viewBox="0 0 20 20" fill="currentColor" class="h-4 w-4" aria-hidden="true">
            <path
              fill-rule="evenodd"
              d="M10 3a.75.75 0 0 1 .75.75v10.19l2.97-2.97a.75.75 0 1 1 1.06 1.06l-4.25 4.25a.75.75 0 0 1-1.06 0L5.22 12.03a.75.75 0 1 1 1.06-1.06l2.97 2.97V3.75A.75.75 0 0 1 10 3Z"
              clip-rule="evenodd"
            />
          </svg>
        </button>
      </Show>
    </div>
  )
}
