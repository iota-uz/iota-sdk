import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { Warning, ArrowClockwise, ArrowsCounterClockwise, ArrowSquareOut, X } from '../icons';
import { useTranslation } from '../hooks/useTranslation';
const OPENAI_BILLING_URL = 'https://platform.openai.com/settings/organization/billing/overview';
const OPENAI_API_KEYS_URL = 'https://platform.openai.com/api-keys';
const OPENAI_STATUS_URL = 'https://status.openai.com';
interface ProviderErrorPresentation {
    titleKey: string;
    descriptionKey: string;
    retryable: boolean;
    adminAction?: boolean;
    action?: {
        href: string;
        labelKey: string;
    };
}
function providerErrorPresentation(error: string): ProviderErrorPresentation | null {
    if (error.includes('provider_billing_balance_exhausted')) {
        return {
            titleKey: 'BiChat.StreamError.ProviderBalance.Title',
            descriptionKey: 'BiChat.StreamError.ProviderBalance.Details',
            retryable: false,
            adminAction: true,
            action: {
                href: OPENAI_BILLING_URL,
                labelKey: 'BiChat.StreamError.OpenBilling',
            },
        };
    }
    if (error.includes('provider_billing_') ||
        error.includes('provider_quota_exhausted') ||
        error.includes('insufficient_quota') ||
        error.includes('credit_balance_exhausted')) {
        return {
            titleKey: 'BiChat.StreamError.ProviderBilling.Title',
            descriptionKey: 'BiChat.StreamError.ProviderBilling.Details',
            retryable: false,
            adminAction: true,
            action: {
                href: OPENAI_BILLING_URL,
                labelKey: 'BiChat.StreamError.OpenBilling',
            },
        };
    }
    if (error.includes('provider_auth_') || error.includes('provider_ip_not_authorized')) {
        return {
            titleKey: 'BiChat.StreamError.ProviderAuth.Title',
            descriptionKey: 'BiChat.StreamError.ProviderAuth.Details',
            retryable: false,
            adminAction: true,
            action: {
                href: OPENAI_API_KEYS_URL,
                labelKey: 'BiChat.StreamError.OpenAPIKeys',
            },
        };
    }
    if (error.includes('provider_permission_denied') ||
        error.includes('provider_region_unsupported')) {
        return {
            titleKey: 'BiChat.StreamError.ProviderAccess.Title',
            descriptionKey: 'BiChat.StreamError.ProviderAccess.Details',
            retryable: false,
            adminAction: true,
        };
    }
    if (error.includes('provider_rate_limited') || error.includes('provider_slow_down')) {
        return {
            titleKey: 'BiChat.StreamError.ProviderRateLimit.Title',
            descriptionKey: 'BiChat.StreamError.ProviderRateLimit.Details',
            retryable: true,
        };
    }
    if (error.includes('provider_context_limit')) {
        return {
            titleKey: 'BiChat.StreamError.ProviderContextLimit.Title',
            descriptionKey: 'BiChat.StreamError.ProviderContextLimit.Details',
            retryable: false,
        };
    }
    if (error.includes('provider_bad_request') ||
        error.includes('provider_unprocessable') ||
        error.includes('provider_not_found')) {
        return {
            titleKey: 'BiChat.StreamError.ProviderRequest.Title',
            descriptionKey: 'BiChat.StreamError.ProviderRequest.Details',
            retryable: false,
            adminAction: true,
        };
    }
    if (error.includes('provider_timeout') || error.includes('provider_connection')) {
        return {
            titleKey: 'BiChat.StreamError.ProviderConnection.Title',
            descriptionKey: 'BiChat.StreamError.ProviderConnection.Details',
            retryable: true,
        };
    }
    if (error.includes('provider_server_error') ||
        error.includes('provider_overloaded') ||
        error.includes('provider_response_failed') ||
        error.includes('provider_conflict')) {
        return {
            titleKey: 'BiChat.StreamError.ProviderUnavailable.Title',
            descriptionKey: 'BiChat.StreamError.ProviderUnavailable.Details',
            retryable: true,
            action: {
                href: OPENAI_STATUS_URL,
                labelKey: 'BiChat.StreamError.OpenStatus',
            },
        };
    }
    if (error.includes('provider_response_incomplete')) {
        return {
            titleKey: 'BiChat.StreamError.ProviderIncomplete.Title',
            descriptionKey: 'BiChat.StreamError.ProviderIncomplete.Details',
            retryable: true,
        };
    }
    return null;
}
interface StreamErrorProps {
    /** Error message to display */
    error: string;
    /** Callback to retry the failed operation */
    onRetry?: () => void;
    /** Callback to regenerate the message */
    onRegenerate?: () => void;
    /** Callback to dismiss the error */
    onDismiss?: () => void;
    /** Whether to show compact mode (less padding) */
    compact?: boolean;
}
export function StreamError(solidProps1Input: StreamErrorProps) {
    const solidProps1 = mergeProps({ compact: false } as const, solidProps1Input);
    const solidState2 = useTranslation();
    const normalizedError = createMemo(() => solidProps1.error.toLowerCase());
    const providerError = createMemo(() => providerErrorPresentation(normalizedError()));
    return (<div class={`flex items-start gap-3 ${solidProps1.compact ? 'px-3 py-2.5' : 'px-4 py-3'} bg-red-50 dark:bg-red-950/40 border border-red-200/80 dark:border-red-900/60 rounded-xl shadow-sm`} role="alert">
      <div class="flex-shrink-0 mt-0.5 flex items-center justify-center w-7 h-7 rounded-full bg-red-100 dark:bg-red-900/40">
        <Warning className="w-4 h-4 text-red-600 dark:text-red-400" weight="fill"/>
      </div>
      <div class="flex-1 min-w-0">
        <p class="text-sm font-medium text-red-800 dark:text-red-200 leading-snug">
          {providerError() ? solidState2.t(providerError()!.titleKey)
            : solidState2.t('BiChat.Error.Generic')}
        </p>
        <p class="mt-0.5 text-xs text-red-600/80 dark:text-red-400/70 break-words leading-relaxed">
          {providerError() ? solidState2.t(providerError()!.descriptionKey)
            : solidProps1.error}
        </p>
        {providerError()?.adminAction && (<p class="mt-1 text-xs font-medium text-red-700 dark:text-red-300 leading-relaxed">
            {solidState2.t('BiChat.StreamError.AdminAction')}
          </p>)}
        <div class="flex items-center gap-2 mt-2">
          {providerError()?.action && (<a href={providerError()!.action!.href} target="_blank" rel="noreferrer" class="cursor-pointer inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-white bg-red-600 hover:bg-red-700 active:bg-red-800 dark:bg-red-700 dark:hover:bg-red-600 rounded-lg transition-colors shadow-sm">
              <ArrowSquareOut className="w-3.5 h-3.5"/>
              {solidState2.t(providerError()!.action!.labelKey)}
            </a>)}
          {solidProps1.onRetry && providerError()?.retryable !== false && (<button onClick={solidProps1.onRetry} class="cursor-pointer inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-white bg-red-600 hover:bg-red-700 active:bg-red-800 dark:bg-red-700 dark:hover:bg-red-600 rounded-lg transition-colors shadow-sm" type="button">
              <ArrowClockwise className="w-3.5 h-3.5"/>
              {solidState2.t('BiChat.StreamError.Retry')}
            </button>)}
          {solidProps1.onRegenerate && (<button onClick={solidProps1.onRegenerate} class="cursor-pointer inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 rounded-lg transition-colors shadow-sm" type="button">
              <ArrowsCounterClockwise className="w-3.5 h-3.5"/>
              {solidState2.t('BiChat.StreamError.Regenerate')}
            </button>)}
        </div>
      </div>
      {solidProps1.onDismiss && (<button onClick={solidProps1.onDismiss} class="cursor-pointer flex-shrink-0 mt-0.5 inline-flex items-center justify-center w-6 h-6 text-red-400 dark:text-red-500 hover:text-red-600 dark:hover:text-red-300 hover:bg-red-100 dark:hover:bg-red-900/40 rounded-md transition-colors" type="button" aria-label={solidState2.t('BiChat.Chat.DismissNotification')}>
          <X className="w-3.5 h-3.5"/>
        </button>)}
    </div>);
}
