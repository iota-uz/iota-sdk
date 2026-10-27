import { createContext, useContext, type Accessor, type JSX } from 'solid-js'
import { en } from './locales/en'
import { ru } from './locales/ru'
import { uz } from './locales/uz'
import type { ResolvedAppContext } from '../context/appContext'

export type Dictionary = Record<string, string>

const DICTIONARIES: Record<string, Dictionary> = {
  en,
  ru,
  uz,
}

export type I18n = {
  language: Accessor<string>
  t: (key: string, vars?: Record<string, string | number>) => string
}

const I18nContext = createContext<I18n>()

export function resolveDictionary(language: string, overrides: Record<string, string>): Dictionary {
  const base = DICTIONARIES[language] ?? en
  return Object.keys(overrides).length > 0 ? { ...base, ...overrides } : base
}

export function createTranslator(ctx: ResolvedAppContext): I18n {
  const language = ctx.locale.language
  const dictionary = resolveDictionary(language, ctx.locale.translations)
  const t = (key: string, vars?: Record<string, string | number>): string => {
    let value = dictionary[key] ?? en[key] ?? key
    if (vars) {
      for (const [name, replacement] of Object.entries(vars)) {
        value = value.replaceAll(`{{${name}}}`, String(replacement))
      }
    }
    return value
  }
  return { language: () => language, t }
}

export function I18nProvider(props: { value: I18n; children: JSX.Element }): JSX.Element {
  return <I18nContext.Provider value={props.value}>{props.children}</I18nContext.Provider>
}

export function useI18n(): I18n {
  const i18n = useContext(I18nContext)
  if (!i18n) throw new Error('useI18n must be used within I18nProvider')
  return i18n
}

export function useTranslation(): { t: I18n['t']; language: Accessor<string> } {
  const i18n = useI18n()
  return { t: i18n.t, language: i18n.language }
}
