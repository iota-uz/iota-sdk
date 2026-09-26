import type { JSX } from 'solid-js'
import { render } from '@solidjs/testing-library'
import { createTranslator, I18nProvider, type I18n } from '../../i18n/i18n'
import type { ResolvedAppContext } from '../../context/appContext'

const TEST_CONTEXT: ResolvedAppContext = {
  config: { basePath: '', rpcUIEndpoint: '/rpc' },
  session: {},
  user: {},
  tenant: {},
  locale: { language: 'en', translations: {} },
  extensions: {},
}

export function createTestI18n(): I18n {
  return createTranslator(TEST_CONTEXT)
}

export function renderWithI18n(ui: () => JSX.Element): ReturnType<typeof render> {
  const i18n = createTestI18n()
  return render(() => <I18nProvider value={i18n}>{ui()}</I18nProvider>)
}
