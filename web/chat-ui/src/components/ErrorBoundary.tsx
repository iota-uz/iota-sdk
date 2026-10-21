import { splitProps } from 'solid-js';
import { ErrorBoundary as SolidErrorBoundary, type JSX } from 'solid-js';
import { WarningCircle, ArrowClockwise } from '../icons';
import { useTranslation } from '../hooks/useTranslation';
export interface ErrorInfo {
    componentStack?: string | null;
}
interface ErrorBoundaryProps {
    children: JSX.Element;
    fallback?: JSX.Element | ((error: Error, reset: () => void) => JSX.Element);
    onError?: (error: Error, info: ErrorInfo) => void;
    emergencyStrings?: {
        title: string;
        fallback: string;
        retry: string;
    };
}
function DefaultErrorContent(rawProps: {
    error: Error | null;
    onReset?: () => void;
    resetLabel?: string;
    errorTitle?: string;
}) {
    const [nativeProps, unusedRest] = splitProps(rawProps, ["error", "onReset", "resetLabel", "errorTitle"]);
    const { t } = useTranslation();
    const resolvedResetLabel = nativeProps.resetLabel ?? t('BiChat.Common.TryAgain');
    const resolvedErrorTitle = nativeProps.errorTitle ?? t('BiChat.Error.SomethingWentWrong');
    return (<div class="flex flex-col items-center justify-center p-8 text-center min-h-[200px]">
      {/* Decorative background pattern */}
      <div class="absolute inset-0 overflow-hidden pointer-events-none opacity-[0.03] dark:opacity-[0.04]" aria-hidden="true">
        <svg class="absolute -top-8 -right-8 w-64 h-64 text-red-500" viewBox="0 0 200 200" fill="currentColor">
          <circle cx="100" cy="100" r="80" opacity="0.5"/>
          <circle cx="100" cy="100" r="50" opacity="0.3"/>
          <circle cx="100" cy="100" r="25" opacity="0.2"/>
        </svg>
      </div>

      <div class="relative flex flex-col items-center">
        {/* Icon with soft glow ring */}
        <div class="relative mb-5">
          <div class="absolute inset-0 rounded-full bg-red-100 dark:bg-red-900/30 scale-150 blur-md"/>
          <div class="relative flex items-center justify-center w-14 h-14 rounded-full bg-red-50 dark:bg-red-900/20 border border-red-200/60 dark:border-red-800/40">
            <WarningCircle size={28} className="text-red-500 dark:text-red-400" weight="fill"/>
          </div>
        </div>

        <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-1.5">{resolvedErrorTitle}</h2>
        <p class="text-sm text-gray-500 dark:text-gray-400 mb-5 max-w-md leading-relaxed">
          {nativeProps.error?.message || t('BiChat.Error.UnexpectedError')}
        </p>

        {nativeProps.onReset && (<button type="button" onClick={nativeProps.onReset} class="flex items-center gap-2 px-5 py-2.5 bg-red-600 hover:bg-red-700 active:bg-red-800 text-white rounded-lg transition-colors shadow-sm text-sm font-medium">
            <ArrowClockwise size={16} weight="bold"/>
            {resolvedResetLabel}
          </button>)}
      </div>
    </div>);
}
/**
 * Hook-free emergency fallback used when the configured fallback crashes.
 */
function StaticEmergencyErrorContent(rawProps: {
    error: Error | null;
    onReset?: () => void;
    titleText?: string;
    fallbackText?: string;
    retryText?: string;
}) {
    const [nativeProps, unusedRest] = splitProps(rawProps, ["error", "onReset", "titleText", "fallbackText", "retryText"]);
    return (<div class="flex flex-col items-center justify-center p-8 text-center min-h-[200px]">
      <div class="relative flex flex-col items-center">
        <div class="relative mb-5">
          <div class="absolute inset-0 rounded-full bg-red-100 scale-150 blur-md"/>
          <div class="relative flex items-center justify-center w-14 h-14 rounded-full bg-red-50 border border-red-200/60">
            <WarningCircle size={28} className="text-red-500" weight="fill"/>
          </div>
        </div>

        <h2 class="text-lg font-semibold text-gray-900 mb-1.5">{nativeProps.titleText || 'Something went wrong'}</h2>
        <p class="text-sm text-gray-500 mb-5 max-w-md leading-relaxed">
          {nativeProps.error?.message || nativeProps.fallbackText || 'An unexpected UI error occurred.'}
        </p>

        {nativeProps.onReset && (<button type="button" onClick={nativeProps.onReset} class="flex items-center gap-2 px-5 py-2.5 bg-red-600 hover:bg-red-700 active:bg-red-800 text-white rounded-lg transition-colors shadow-sm text-sm font-medium">
            <ArrowClockwise size={16} weight="bold"/>
            {nativeProps.retryText || 'Try again'}
          </button>)}
      </div>
    </div>);
}
function ErrorBoundary(props: ErrorBoundaryProps) {
    const report = (cause: unknown) => {
        const error = cause instanceof Error ? cause : new Error(String(cause));
        console.error('Ali UI boundary caught an error', error);
        props.onError?.(error, {});
        return error;
    };
    return <SolidErrorBoundary fallback={(cause, reset) => {
            const error = report(cause);
            return <SolidErrorBoundary fallback={secondary => {
                    report(secondary);
                    return <StaticEmergencyErrorContent error={error} onReset={reset} titleText={props.emergencyStrings?.title} fallbackText={props.emergencyStrings?.fallback} retryText={props.emergencyStrings?.retry}/>;
                }}>
    {typeof props.fallback === 'function' ? props.fallback(error, reset) : props.fallback ?? <DefaultErrorContent error={error} onReset={reset}/>}
   </SolidErrorBoundary>;
        }}>{props.children}</SolidErrorBoundary>;
}
export default ErrorBoundary;
export { ErrorBoundary, DefaultErrorContent };
