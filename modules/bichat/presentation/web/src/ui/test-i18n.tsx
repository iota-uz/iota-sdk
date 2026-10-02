import { render } from '@solidjs/testing-library'
import type { JSX } from 'solid-js'
import { createTranslator, I18nProvider } from '../i18n/i18n'
import type { ResolvedAppContext } from '../context/appContext'

const baseContext: ResolvedAppContext = {
  config: { basePath: '', rpcUIEndpoint: '/rpc' },
  session: {},
  user: {},
  tenant: {},
  locale: { language: 'en', translations: {} },
  extensions: {},
}

export function renderWithI18n(ui: () => JSX.Element, translations: Record<string, string> = {}) {
  const translator = createTranslator({
    ...baseContext,
    locale: { language: 'en', translations },
  })
  return render(() => <I18nProvider value={translator}>{ui()}</I18nProvider>)
}
