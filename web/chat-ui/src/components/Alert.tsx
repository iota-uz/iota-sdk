import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { X, Warning, CheckCircle, Info, XCircle } from '../icons';
import { errorMessageVariants } from '../animations/variants';
import { useTranslation } from '../hooks/useTranslation';
export type AlertVariant = 'error' | 'success' | 'warning' | 'info';
interface AlertProps {
    variant?: AlertVariant;
    message: string;
    title?: string;
    onDismiss?: () => void;
    onRetry?: () => void;
    show?: boolean;
    dismissible?: boolean;
}
const variantStyles = {
    error: {
        container: 'border-red-200 bg-red-50 dark:bg-red-900/20',
        title: 'text-red-800 dark:text-red-300',
        message: 'text-red-700 dark:text-red-400',
        icon: 'text-red-600 dark:text-red-400',
        button: 'text-red-400 hover:text-red-600 dark:hover:text-red-300',
        retryButton: 'bg-red-600 dark:bg-red-700 hover:bg-red-700 dark:hover:bg-red-800 text-white',
        Icon: XCircle,
    },
    success: {
        container: 'border-emerald-200 bg-emerald-50 dark:bg-emerald-900/20',
        title: 'text-emerald-800 dark:text-emerald-300',
        message: 'text-emerald-700 dark:text-emerald-400',
        icon: 'text-emerald-600 dark:text-emerald-400',
        button: 'text-emerald-400 hover:text-emerald-600 dark:hover:text-emerald-300',
        retryButton: 'bg-emerald-600 dark:bg-emerald-700 hover:bg-emerald-700 dark:hover:bg-emerald-800 text-white',
        Icon: CheckCircle,
    },
    warning: {
        container: 'border-amber-200 bg-amber-50 dark:bg-amber-900/20',
        title: 'text-amber-800 dark:text-amber-300',
        message: 'text-amber-700 dark:text-amber-400',
        icon: 'text-amber-600 dark:text-amber-400',
        button: 'text-amber-400 hover:text-amber-600 dark:hover:text-amber-300',
        retryButton: 'bg-amber-600 dark:bg-amber-700 hover:bg-amber-700 dark:hover:bg-amber-800 text-white',
        Icon: Warning,
    },
    info: {
        container: 'border-blue-200 bg-blue-50 dark:bg-blue-900/20',
        title: 'text-blue-800 dark:text-blue-300',
        message: 'text-blue-700 dark:text-blue-400',
        icon: 'text-blue-600 dark:text-blue-400',
        button: 'text-blue-400 hover:text-blue-600 dark:hover:text-blue-300',
        retryButton: 'bg-blue-600 dark:bg-blue-700 hover:bg-blue-700 dark:hover:bg-blue-800 text-white',
        Icon: Info,
    },
};
function Alert(solidProps1Input: AlertProps) {
    const solidProps1 = mergeProps({ variant: 'info', show: true, dismissible: true } as const, solidProps1Input);
    const solidState2 = useTranslation();
    const styles = createMemo(() => variantStyles[solidProps1.variant]);
    const IconComponent = styles().Icon;
    return (<>
      {solidProps1.show && (<div class={`border-t border ${styles().container} px-4 py-3`} role="alert" aria-live="assertive">
          <div class="w-full flex items-start justify-between px-4">
            <div class="flex items-start gap-3 flex-1">
              {/* Icon */}
              <IconComponent size={20} className={`w-5 h-5 ${styles().icon} flex-shrink-0 mt-0.5`}/>

              {/* Content */}
              <div class="flex-1">
                {solidProps1.title && <p class={`text-sm ${styles().title} font-medium`}>{solidProps1.title}</p>}
                <p class={`text-sm ${styles().message} ${solidProps1.title ? 'mt-1' : ''}`}>{solidProps1.message}</p>

                {/* Retry Button */}
                {solidProps1.onRetry && (<button onClick={solidProps1.onRetry} class={`mt-2 text-xs px-3 py-1.5 rounded ${styles().retryButton} transition-colors font-medium`}>
                    {solidState2.t('BiChat.Chat.Retry')}
                  </button>)}
              </div>
            </div>

            {/* Dismiss Button */}
            {solidProps1.dismissible && solidProps1.onDismiss && (<button onClick={solidProps1.onDismiss} class={`${styles().button} transition-colors flex-shrink-0`} aria-label={solidState2.t('BiChat.Chat.DismissNotification')}>
                <X size={20} className="w-5 h-5"/>
              </button>)}
          </div>
        </div>)}
    </>);
}
export default Alert;
