import { For, Show, createSignal, type JSX } from 'solid-js'
import type { PendingQuestion } from '../types'
import type { PendingQuestionItem } from '../../rpc.generated'
import { useI18n } from '../../i18n/i18n'

function QuestionField(props: {
  item: PendingQuestionItem
  value: string
  invalid: boolean
  onInput: (value: string) => void
}): JSX.Element {
  const i18n = useI18n()
  const commonClass = () =>
    `w-full rounded-lg border px-3 py-2 text-sm focus:outline-none focus:ring-1 ${
      props.invalid
        ? 'border-red-300 focus:border-red-500 focus:ring-red-500'
        : 'border-neutral-300 focus:border-sky-500 focus:ring-sky-500'
    }`
  const isSelect = (): boolean => props.item.type === 'select' || props.item.options.length > 0
  return (
    <div class="space-y-1">
      <label for={`hitl-${props.item.id}`} class="block text-sm font-medium text-neutral-700">
        {props.item.text}
      </label>
      <Show
        when={isSelect()}
        fallback={
          <Show
            when={props.item.type === 'textarea'}
            fallback={
              <input
                id={`hitl-${props.item.id}`}
                type="text"
                class={commonClass()}
                value={props.value}
                onInput={(event) => props.onInput(event.currentTarget.value)}
              />
            }
          >
            <textarea
              id={`hitl-${props.item.id}`}
              rows={3}
              class={commonClass()}
              value={props.value}
              onInput={(event) => props.onInput(event.currentTarget.value)}
            />
          </Show>
        }
      >
        <select
          id={`hitl-${props.item.id}`}
          class={commonClass()}
          value={props.value}
          onChange={(event) => props.onInput(event.currentTarget.value)}
        >
          <option value="" />
          <For each={props.item.options}>{(option) => <option value={option.id}>{option.label}</option>}</For>
        </select>
      </Show>
      <Show when={props.invalid}>
        <p class="text-xs text-red-600">{i18n.t('hitl.questionRequired')}</p>
      </Show>
    </div>
  )
}

export function HITLForm(props: {
  question: PendingQuestion
  onSubmit: (answers: Record<string, string>) => void
  onReject: () => void
}): JSX.Element {
  const i18n = useI18n()
  const [answers, setAnswers] = createSignal<Record<string, string>>({})
  const [attempted, setAttempted] = createSignal(false)

  const setAnswer = (id: string, value: string): void => {
    setAnswers((current) => ({ ...current, [id]: value }))
  }

  const missingRequired = (): string[] =>
    props.question.questions
      .filter((item) => !(answers()[item.id] ?? '').trim())
      .map((item) => item.id)

  const invalid = (item: PendingQuestionItem): boolean => attempted() && missingRequired().includes(item.id)

  const submit = (): void => {
    if (missingRequired().length > 0) {
      setAttempted(true)
      return
    }
    props.onSubmit({ ...answers() })
  }

  return (
    <section class="shrink-0 border-t border-neutral-200 bg-white px-4 py-3">
      <h2 class="mb-2 text-sm font-semibold text-neutral-900">{i18n.t('hitl.answer')}</h2>
      <div class="space-y-3">
        <For each={props.question.questions}>
          {(item) => (
            <QuestionField
              item={item}
              value={answers()[item.id] ?? ''}
              invalid={invalid(item)}
              onInput={(value) => setAnswer(item.id, value)}
            />
          )}
        </For>
      </div>
      <div class="mt-3 flex items-center gap-2">
        <button
          type="button"
          class="rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:bg-neutral-300"
          disabled={attempted() && missingRequired().length > 0}
          onClick={submit}
        >
          {i18n.t('hitl.submit')}
        </button>
        <button
          type="button"
          class="rounded-lg border border-red-200 px-4 py-2 text-sm font-medium text-red-600 transition hover:bg-red-50"
          onClick={() => props.onReject()}
        >
          {i18n.t('hitl.reject')}
        </button>
      </div>
    </section>
  )
}
