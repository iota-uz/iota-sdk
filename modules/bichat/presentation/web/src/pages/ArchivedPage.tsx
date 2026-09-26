import { useNavigate } from '@solidjs/router'
import { createResource, For, Show, type JSX } from 'solid-js'
import { useAppContext } from '../context/appContext'
import { createBichatRPCClient, RPCError } from '../rpc/client'
import type { Session } from '../rpc.generated'
import { useAppToast } from '../ui/toast'
import { useI18n } from '../i18n/i18n'

function formatSessionDate(value: string): string {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return ''
  return parsed.toLocaleDateString()
}

export function ArchivedPage(): JSX.Element {
  const ctx = useAppContext()
  const i18n = useI18n()
  const navigate = useNavigate()
  const toast = useAppToast()
  const rpc = createBichatRPCClient(ctx)

  const [archived, { refetch }] = createResource(async () => {
    const result = await rpc.call('bichat.session.list', { limit: 200, offset: 0, includeArchived: true })
    return result.sessions.filter((session) => session.status === 'archived')
  })

  const unarchive = async (session: Session): Promise<void> => {
    try {
      await rpc.call('bichat.session.unarchive', { id: session.id })
      toast.success(i18n.t('archived.unarchive'))
      void refetch()
    } catch (cause) {
      toast.error(cause instanceof RPCError ? cause.message : String(cause))
    }
  }

  return (
    <div class="mx-auto flex w-full max-w-3xl flex-1 flex-col gap-4 overflow-y-auto p-6">
      <h1 class="text-lg font-semibold text-neutral-900">{i18n.t('archived.title')}</h1>
      <Show
        when={archived()}
        fallback={<p class="text-sm text-neutral-500">{i18n.t('archived.empty')}</p>}
      >
        {(sessions) => (
          <Show
            when={sessions().length > 0}
            fallback={<p class="text-sm text-neutral-500">{i18n.t('archived.empty')}</p>}
          >
            <ul class="divide-y divide-neutral-100 rounded-xl border border-neutral-200 bg-white">
              <For each={sessions()}>
                {(session) => (
                  <li class="flex items-center justify-between gap-3 px-4 py-3">
                    <button
                      type="button"
                      class="min-w-0 flex-1 text-left"
                      onClick={() => void navigate(`/session/${session.id}`)}
                    >
                      <span class="block truncate text-sm font-medium text-neutral-900">
                        {session.title}
                      </span>
                      <span class="text-xs text-neutral-500">{formatSessionDate(session.updatedAt)}</span>
                    </button>
                    <button
                      type="button"
                      class="rounded-lg px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-100"
                      onClick={() => void unarchive(session)}
                    >
                      {i18n.t('archived.unarchive')}
                    </button>
                  </li>
                )}
              </For>
            </ul>
          </Show>
        )}
      </Show>
    </div>
  )
}
