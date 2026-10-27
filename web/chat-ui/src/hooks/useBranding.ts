/** @jsxImportSource solid-js */
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * Branding hook for UI customization.
 *
 * Provides access to branding configuration injected from the backend
 * via window.__APPLET_CONTEXT__.extensions.branding
 */
import { useIotaContext } from '../context/IotaContext';
import type { BrandingConfig, ExamplePrompt } from '../types';
import { useTranslation } from './useTranslation';
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
];
/**
 * Hook to access branding configuration.
 *
 * Returns merged branding with fallbacks to defaults and translations.
 */
export function useBranding() {
    const context = useIotaContext();
    const solidState1 = useTranslation();
    const branding = createMemo((): BrandingConfig => {
        const customBranding = context.extensions?.branding || {};
        // Get example prompts with category translations
        let examplePrompts = customBranding.welcome?.examplePrompts;
        if (!examplePrompts || examplePrompts.length === 0) {
            // Use defaults with translated categories
            examplePrompts = defaultExamplePrompts.map((p) => ({
                ...p,
                category: solidState1.t(`category.${p.category.toLowerCase().replace(/\s+/g, '')}`) || p.category,
            }));
        }
        return {
            appName: customBranding.appName || 'BiChat',
            logoUrl: customBranding.logoUrl,
            welcome: {
                title: customBranding.welcome?.title || solidState1.t('BiChat.Welcome.Title'),
                description: customBranding.welcome?.description || solidState1.t('BiChat.Welcome.Description'),
                examplePrompts,
            },
            theme: customBranding.theme,
        };
    });
    return branding();
}
/**
 * Hook to access feature flags.
 */
export function useFeatureFlags() {
    const context = useIotaContext();
    return createMemo(() => ({
        vision: context.extensions?.features?.vision ?? false,
        webSearch: context.extensions?.features?.webSearch ?? false,
        codeInterpreter: context.extensions?.features?.codeInterpreter ?? false,
        multiAgent: context.extensions?.features?.multiAgent ?? false,
    }))();
}
