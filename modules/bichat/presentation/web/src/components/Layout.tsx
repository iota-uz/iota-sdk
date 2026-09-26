import { useNavigate } from '@solidjs/router'
import { createEffect, createSignal, onCleanup, onMount, Show, type JSX } from 'solid-js'
import { useI18n } from '../i18n/i18n'
import { PanelLeftIcon, XIcon } from '../ui/icons'
import { SkipLink } from '../ui/primitives'
import { Sidebar } from './Sidebar'

export function Layout(props: { children: JSX.Element }): JSX.Element {
  const { t } = useI18n()
  const navigate = useNavigate()
  const [drawerOpen, setDrawerOpen] = createSignal(false)
  const [drawerMounted, setDrawerMounted] = createSignal(false)
  let closeButton: HTMLButtonElement | undefined

  createEffect(() => {
    if (drawerOpen()) {
      setDrawerMounted(true)
      queueMicrotask(() => closeButton?.focus())
    }
  })

  onMount(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && drawerOpen()) {
        event.preventDefault()
        setDrawerOpen(false)
        return
      }
      if ((event.metaKey || event.ctrlKey) && (event.key === 'n' || event.key === 'N')) {
        event.preventDefault()
        navigate('/')
      }
    }
    document.addEventListener('keydown', onKeyDown)
    onCleanup(() => document.removeEventListener('keydown', onKeyDown))
  })

  return (
    <div class="relative flex h-full w-full min-h-0 flex-1 overflow-hidden">
      <SkipLink />

      {/* Sidebar — desktop */}
      <div class="hidden md:block">
        <Sidebar />
      </div>

      {/* Sidebar — mobile drawer */}
      <Show when={drawerMounted()}>
        <div
          class={`fixed inset-0 z-40 bg-black/40 transition-opacity duration-200 md:hidden ${
            drawerOpen() ? 'opacity-100' : 'pointer-events-none opacity-0'
          }`}
          aria-hidden="true"
          onClick={() => setDrawerOpen(false)}
        />
        <div
          id="bichat-drawer"
          aria-hidden={!drawerOpen()}
          class={`fixed inset-y-0 left-0 z-50 w-72 max-w-[85vw] bg-white shadow-2xl transition-transform duration-200 md:hidden ${
            drawerOpen() ? 'translate-x-0' : 'pointer-events-none -translate-x-full'
          }`}
        >
          <div class="flex h-full flex-col">
            <div class="flex items-center justify-between border-b border-neutral-100 px-3 py-2">
              <span class="text-sm font-semibold text-neutral-800">{t('nav.chats')}</span>
              <button
                ref={closeButton}
                type="button"
                onClick={() => setDrawerOpen(false)}
                aria-label={t('nav.closeMenu')}
                class="cursor-pointer rounded-lg p-1.5 text-neutral-500 transition-colors hover:bg-neutral-100 hover:text-neutral-700"
              >
                <XIcon class="h-5 w-5" />
              </button>
            </div>
            <div class="min-h-0 flex-1">
              <Sidebar onNavigate={() => setDrawerOpen(false)} />
            </div>
          </div>
        </div>
      </Show>

      {/* Main content */}
      <main id="bichat-main" class="relative flex min-h-0 flex-1 flex-col overflow-hidden">
        <div class="flex items-center gap-2 border-b border-neutral-200 bg-white px-3 py-2 md:hidden">
          <button
            type="button"
            onClick={() => setDrawerOpen(true)}
            aria-label={t('nav.openMenu')}
            aria-expanded={drawerOpen()}
            aria-controls="bichat-drawer"
            class="cursor-pointer rounded-lg p-1.5 text-neutral-600 transition-colors hover:bg-neutral-100"
          >
            <PanelLeftIcon class="h-5 w-5" />
          </button>
          <span class="text-sm font-semibold text-neutral-800">{t('nav.chats')}</span>
        </div>
        <div class="flex min-h-0 flex-1 flex-col">{props.children}</div>
      </main>
    </div>
  )
}
