import { ErrorBoundary as SolidErrorBoundary, type JSX } from 'solid-js'
import { useI18n } from '../i18n/i18n'

export function AppErrorBoundary(props: { children: JSX.Element }): JSX.Element {
  const { t } = useI18n()
  return (
    <SolidErrorBoundary
      fallback={(error) => (
        <div class="flex h-full w-full items-center justify-center p-6">
          <div class="w-full max-w-md rounded-2xl border border-neutral-200 bg-white p-6 text-center shadow-sm">
            <h2 class="text-base font-semibold text-neutral-900">{t('error.title')}</h2>
            <p class="mt-2 text-sm break-words text-neutral-500">
              {error instanceof Error ? error.message : String(error)}
            </p>
            <button
              type="button"
              onClick={() => window.location.reload()}
              class="mt-4 cursor-pointer rounded-lg bg-neutral-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-neutral-700"
            >
              {t('error.reload')}
            </button>
          </div>
        </div>
      )}
    >
      {props.children}
    </SolidErrorBoundary>
  )
}
