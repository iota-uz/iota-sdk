// Locale helpers. Every module keeps its strings in a table keyed by EditorLocale with all four locales;
// names of nodes and events come already localized from the catalog.
import type { EditorLocale } from './types'

export const LOCALES: readonly EditorLocale[] = ['en', 'ru', 'uz', 'uz-Cyrl']

export function pick<T>(table: Record<EditorLocale, T>, locale: EditorLocale | undefined): T {
  return table[locale ?? 'en'] ?? table.en
}

/** BCP 47 tag for Intl APIs. */
export function intlLocale(locale: EditorLocale | undefined): string {
  switch (locale) {
    case 'ru': return 'ru-RU'
    case 'uz': return 'uz-Latn-UZ'
    case 'uz-Cyrl': return 'uz-Cyrl-UZ'
    default: return 'en-US'
  }
}

/** Plural category by the language's rules (en/ru/uz). */
export function plural(locale: EditorLocale | undefined, count: number, forms: { one: string; few?: string; many?: string; other: string }): string {
  const category = new Intl.PluralRules(intlLocale(locale)).select(count)
  const form = (forms as Record<string, string | undefined>)[category] ?? forms.other
  return form.replace('{n}', String(count))
}

export function isMac(): boolean {
  if (typeof navigator === 'undefined') return false
  const platform = (navigator as Navigator & { userAgentData?: { platform?: string } }).userAgentData?.platform ?? navigator.platform ?? ''
  return /mac|iphone|ipad|ipod/i.test(platform)
}
