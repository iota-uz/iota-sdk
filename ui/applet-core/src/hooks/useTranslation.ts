import { useAppletContext } from '../context/AppletContext'
import type { TranslationHook } from '../types'

/**
 * useTranslation provides i18n translation utilities.
 * All translations are automatically passed from backend locale bundle.
 *
 * Usage:
 * const { t, language } = useTranslation()
 *
 * // Simple translation
 * t('BiChat.Title') // Returns translated text
 *
 * // Translation with interpolation
 * t('Common.WelcomeMessage', { name: 'John' })
 * // If translation is "Welcome {name}!" -> Returns "Welcome John!"
 *
 * React uses same keys as Go backend:
 * Go:    pageCtx.T("BiChat.Title")
 * React: t("BiChat.Title")
 */
export function useTranslation(): TranslationHook {
  const { locale } = useAppletContext()

  const t = (key: string, params?: Record<string, unknown>): string => {
    let text = locale.translations[key] || key

    // Simple interpolation: "Hello {name}" with {name: "World"}
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        text = text.replace(new RegExp(`\\{${k}\\}`, 'g'), String(v))
      })
    }

    return text
  }

  return {
    t,
    language: locale.language
  }
}
