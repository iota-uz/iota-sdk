import { For, Show, type JSX } from 'solid-js'
import { useI18n } from '../../i18n/i18n'

const SUGGESTIONS: string[] = [
  'Show revenue by month for this year',
  'What are my top 10 customers by sales?',
  'Compare sales this month versus last month',
  'Which products have the lowest stock right now?',
]

export function WelcomeContent(props: {
  visible: () => boolean
  onPick: (prompt: string) => void
}): JSX.Element {
  const i18n = useI18n()
  return (
    <Show when={props.visible()}>
      <div class="flex flex-1 flex-col items-center justify-center px-6 py-10">
        <h2 class="text-2xl font-semibold tracking-tight text-neutral-900">
          {i18n.t('welcome.title')}
        </h2>
        <p class="mt-2 text-sm text-neutral-500">{i18n.t('welcome.subtitle')}</p>
        <div class="mt-6 grid w-full max-w-xl grid-cols-1 gap-2 sm:grid-cols-2">
          <For each={SUGGESTIONS}>
            {(prompt) => (
              <button
                type="button"
                class="rounded-lg border border-neutral-200 bg-white px-3 py-2 text-left text-sm text-neutral-700 transition hover:border-neutral-300 hover:bg-neutral-50"
                onClick={() => props.onPick(prompt)}
              >
                {prompt}
              </button>
            )}
          </For>
        </div>
      </div>
    </Show>
  )
}
