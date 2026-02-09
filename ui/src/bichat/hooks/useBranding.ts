/**
 * Branding hook for UI customization.
 *
 * Provides access to branding configuration injected from the backend
 * via window.__BICHAT_CONTEXT__.extensions.branding
 */

import { useMemo } from 'react'
import { useIotaContext } from '../context/IotaContext'
import type { BrandingConfig, ExamplePrompt } from '../types'
import { useTranslation } from './useTranslation'

/**
 * Default example prompts when none are configured.
 */
const defaultExamplePrompts: ExamplePrompt[] = [
  {
    category: 'Data Analysis',
    text: 'Show me sales trends for the last quarter',
    icon: 'chart-bar',
  },
  {
    category: 'Reports',
    text: 'Generate a summary of recent activity',
    icon: 'file-text',
  },
  {
    category: 'Insights',
    text: 'What are the top performing items?',
    icon: 'lightbulb',
  },
]

/**
 * Hook to access branding configuration.
 *
 * Returns merged branding with fallbacks to defaults and translations.
 */
export function useBranding() {
  const context = useIotaContext()
  const { t } = useTranslation()

  const branding = useMemo((): BrandingConfig => {
    const customBranding = context.extensions?.branding || {}

    // Get example prompts with category translations
    let examplePrompts = customBranding.welcome?.examplePrompts
    if (!examplePrompts || examplePrompts.length === 0) {
      // Use defaults with translated categories
      examplePrompts = defaultExamplePrompts.map((p) => ({
        ...p,
        category: t(`category.${p.category.toLowerCase().replace(/\s+/g, '')}`) || p.category,
      }))
    }

    return {
      appName: customBranding.appName || 'BiChat',
      logoUrl: customBranding.logoUrl,
      welcome: {
        title: customBranding.welcome?.title || t('welcome.title'),
        description: customBranding.welcome?.description || t('welcome.description'),
        examplePrompts,
      },
      theme: customBranding.theme,
    }
  }, [context.extensions?.branding, t])

  return branding
}

/**
 * Hook to access feature flags.
 */
export function useFeatureFlags() {
  const context = useIotaContext()

  return useMemo(
    () => ({
      vision: context.extensions?.features?.vision ?? false,
      webSearch: context.extensions?.features?.webSearch ?? false,
      codeInterpreter: context.extensions?.features?.codeInterpreter ?? false,
      multiAgent: context.extensions?.features?.multiAgent ?? false,
    }),
    [context.extensions?.features]
  )
}
