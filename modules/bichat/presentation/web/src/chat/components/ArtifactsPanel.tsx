import { For, Show, type JSX } from 'solid-js'
import type { Accessor } from 'solid-js'
import type { Artifact as SessionArtifact } from '../../rpc.generated'
import { useI18n } from '../../i18n/i18n'

function FileIcon(props: { type: string }): JSX.Element {
  return (
    <svg
      viewBox="0 0 20 20"
      fill="none"
      stroke="currentColor"
      stroke-width="1.5"
      class="h-5 w-5 shrink-0 text-neutral-500"
      aria-hidden="true"
    >
      {props.type === 'chart' ? (
        <g>
          <path d="M4 16.5v-5M8 16.5v-9M12 16.5v-7M16 16.5v-11" stroke-linecap="round" />
        </g>
      ) : props.type === 'code' ? (
        <g>
          <path d="M4 2.5h8L16 6.5v11H4z" stroke-linejoin="round" />
          <path d="M12 2.5v4h4" stroke-linejoin="round" />
        </g>
      ) : (
        <g>
          <rect x="3" y="4" width="14" height="12" rx="1.5" />
          <path d="M6.5 9l2 2 3-4 2 3" stroke-linecap="round" stroke-linejoin="round" />
        </g>
      )}
    </svg>
  )
}

function formatSize(sizeBytes: number): string {
  return `${Math.max(1, Math.round(sizeBytes / 1024))} KB`
}

export function ArtifactsPanel(props: {
  artifacts: Accessor<SessionArtifact[]>
  expanded: () => boolean
  onToggle: () => void
}): JSX.Element {
  const i18n = useI18n()
  return (
    <Show when={props.expanded()}>
      <aside class="flex w-80 shrink-0 flex-col border-l border-neutral-200 bg-white">
        <div class="flex items-center justify-between border-b border-neutral-200 px-4 py-3">
          <h2 class="text-sm font-semibold text-neutral-900">{i18n.t('artifacts.title')}</h2>
          <button
            type="button"
            aria-label={i18n.t('artifacts.close')}
            class="rounded-md p-1 text-neutral-500 transition hover:bg-neutral-100 hover:text-neutral-700"
            onClick={() => props.onToggle()}
          >
            <svg viewBox="0 0 20 20" fill="currentColor" class="h-4 w-4" aria-hidden="true">
              <path d="M6.28 5.22a.75.75 0 0 0-1.06 1.06L8.94 10l-3.72 3.72a.75.75 0 1 0 1.06 1.06L10 11.06l3.72 3.72a.75.75 0 1 0 1.06-1.06L11.06 10l3.72-3.72a.75.75 0 0 0-1.06-1.06L10 8.94 6.28 5.22Z" />
            </svg>
          </button>
        </div>
        <Show
          when={props.artifacts().length > 0}
          fallback={<p class="px-4 py-6 text-center text-xs text-neutral-400">{i18n.t('artifacts.empty')}</p>}
        >
          <ul class="flex-1 divide-y divide-neutral-100 overflow-y-auto">
            <For each={props.artifacts()}>
              {(artifact) => (
                <li class="flex items-center gap-3 px-4 py-3">
                  <FileIcon type={artifact.type} />
                  <div class="min-w-0 flex-1">
                    <p class="truncate text-sm text-neutral-800" title={artifact.name}>
                      {artifact.name}
                    </p>
                    <p class="text-xs text-neutral-400">{formatSize(artifact.sizeBytes)}</p>
                  </div>
                  <Show when={artifact.url}>
                    <a
                      href={artifact.url}
                      download={artifact.name}
                      aria-label={i18n.t('artifacts.download')}
                      class="rounded-md p-1.5 text-neutral-500 transition hover:bg-neutral-100 hover:text-neutral-700"
                    >
                      <svg
                        viewBox="0 0 20 20"
                        fill="currentColor"
                        class="h-4 w-4"
                        aria-hidden="true"
                      >
                        <path d="M10.75 2.75a.75.75 0 0 0-1.5 0v8.614L6.295 8.235a.75.75 0 1 0-1.09 1.03l4.25 4.5a.75.75 0 0 0 1.09 0l4.25-4.5a.75.75 0 0 0-1.09-1.03l-2.955 3.129V2.75Z" />
                        <path d="M3.5 12.75a.75.75 0 0 0-1.5 0v2.5A2.75 2.75 0 0 0 4.75 18h10.5A2.75 2.75 0 0 0 18 15.25v-2.5a.75.75 0 0 0-1.5 0v2.5c0 .69-.56 1.25-1.25 1.25H4.75c-.69 0-1.25-.56-1.25-1.25v-2.5Z" />
                      </svg>
                    </a>
                  </Show>
                </li>
              )}
            </For>
          </ul>
        </Show>
      </aside>
    </Show>
  )
}
