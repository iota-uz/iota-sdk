import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { Archive, Spinner } from '../icons';
import { errorMessageVariants } from '../animations/variants';
import Alert from './Alert';
import { useTranslation } from '../hooks/useTranslation';
interface ArchiveBannerProps {
    show?: boolean;
    onRestore?: () => Promise<void>;
    restoring?: boolean;
    onRestoreComplete?: () => void;
}
function ArchiveBanner(solidProps1Input: ArchiveBannerProps) {
    const solidProps1 = mergeProps({ show: true, restoring: false } as const, solidProps1Input);
    const solidState2 = useTranslation();
    const [error, setError] = createSignal<string | null>(null);
    const handleRestore = async () => {
        try {
            setError(null);
            if (solidProps1.onRestore) {
                await solidProps1.onRestore?.();
            }
            if (solidProps1.onRestoreComplete) {
                solidProps1.onRestoreComplete?.();
            }
        }
        catch (err) {
            const message = err instanceof Error ? err.message : solidState2.t('BiChat.Archive.RestoreFailed');
            setError(message);
        }
    };
    return (<>
      <>
        {solidProps1.show && (<div class="border-t border border-blue-200 bg-blue-50 dark:bg-blue-900/20 px-4 py-3" role="region" aria-label={solidState2.t('BiChat.Archive.Banner')}>
            <div class="w-full flex items-start justify-between px-4">
              <div class="flex items-start gap-3 flex-1">
                {/* Icon */}
                <Archive size={20} className="w-5 h-5 text-blue-600 dark:text-blue-400 flex-shrink-0 mt-0.5"/>

                {/* Content */}
                <div class="flex-1">
                  <p class="text-sm text-blue-700 dark:text-blue-400">
                    {solidState2.t('BiChat.Archive.Archived')}
                  </p>
                </div>
              </div>

              {/* Restore Button */}
              <button onClick={handleRestore} disabled={solidProps1.restoring} class="ml-2 flex-shrink-0 px-3 py-1.5 text-xs font-medium bg-blue-600 dark:bg-blue-700 hover:bg-blue-700 dark:hover:bg-blue-800 text-white rounded transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1.5" aria-label={solidState2.t('BiChat.Archive.Restore')}>
                {solidProps1.restoring ? (<>
                    <Spinner size={16} className="w-4 h-4 animate-spin"/>
                    {solidState2.t('BiChat.Archive.Restoring')}
                  </>) : (solidState2.t('BiChat.Archive.Restore'))}
              </button>
            </div>
          </div>)}
      </>

      {/* Error Alert */}
      {error() && (<Alert variant="error" message={error()!} title={solidState2.t('BiChat.Archive.RestoreFailed')} onDismiss={() => setError(null)} dismissible/>)}
    </>);
}
export default ArchiveBanner;
