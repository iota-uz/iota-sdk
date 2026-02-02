/**
 * Translation hook using locale from IotaContext
 *
 * Translations are loaded in this priority:
 * 1. extensions.translations (injected by BiChat module with domain customization)
 * 2. locale.translations (SDK-level translations)
 * 3. Default English fallback
 */

import { useIotaContext } from '../context/IotaContext'
import { defaultTranslations } from '../locales/defaults'

export function useTranslation() {
  const context = useIotaContext()
  const { locale, extensions } = context

  /**
   * Get translations from best available source
   */
  const translations = {
    ...defaultTranslations, // Fallback defaults
    ...locale.translations, // SDK-level translations
    ...(extensions?.translations || {}), // BiChat customizations (highest priority)
  }

  /**
   * Translate a key with optional parameter interpolation
   * @param key - Translation key (e.g., 'welcome.title', 'chat.newChat')
   * @param params - Optional parameters for interpolation (e.g., { count: 5 })
   * @returns Translated string
   */
  const t = (key: string, params?: Record<string, string | number>): string => {
    let text = translations[key] || key

    // Simple interpolation: replace {key} with params[key]
    if (params) {
      Object.keys(params).forEach((paramKey) => {
        const value = params[paramKey]
        text = text.replace(new RegExp(`\\{${paramKey}\\}`, 'g'), String(value))
      })
    }

    return text
  }

  return {
    t,
    locale: locale.language,
    translations,
  }
}
