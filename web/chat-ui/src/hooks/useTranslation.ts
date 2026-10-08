/** @jsxImportSource solid-js */
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * Translation hook using locale from IotaContext
 */
import { useIotaContext } from '../context/IotaContext';
export function useTranslation() {
    const solidState1 = useIotaContext();
    const { translations, language } = solidState1.locale;
    /**
     * Translate a key with optional parameter interpolation
     * @param key - Translation key (e.g., 'bichat.title')
     * @param params - Optional parameters for interpolation (e.g., { name: 'John' })
     * @returns Translated string
     */
    const t = (key: string, params?: Record<string, string | number | boolean>): string => {
        const raw = translations[key];
        let text = typeof raw === 'string' && raw.trim() ? raw : key;
        // Simple interpolation: replace {{key}} with params[key]
        if (params) {
            Object.keys(params).forEach((paramKey) => {
                const value = params[paramKey];
                text = text.replace(new RegExp(`{{${paramKey}}}`, 'g'), String(value));
            });
        }
        return text.trim() ? text : key;
    };
    return {
        t,
        locale: language,
    };
}
