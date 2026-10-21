import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
// Shadow-DOM-safe dialog: Headless UI Dialog portals to document.body, escaping
// the shadow root and dropping all scoped Tailwind. InlineDialog stays inline.
import { InlineDialog, InlineDialogBackdrop, InlineDialogPanel } from './InlineDialog';
import { FloppyDisk, PencilSimple, Trash, X } from '../icons';
import type { SessionArtifact } from '../types';
import { useTranslation } from '../hooks/useTranslation';
import { SessionArtifactPreview } from './SessionArtifactPreview';
interface SessionArtifactPreviewModalProps {
    isOpen: boolean;
    artifact: SessionArtifact | null;
    canRename?: boolean;
    canDelete?: boolean;
    onClose: () => void;
    onRename?: (artifact: SessionArtifact, name: string) => Promise<void>;
    onDelete?: (artifact: SessionArtifact) => Promise<void>;
}
export function SessionArtifactPreviewModal(solidProps1Input: SessionArtifactPreviewModalProps) {
    const solidProps1 = mergeProps({ canRename: false, canDelete: false } as const, solidProps1Input);
    const solidState2 = useTranslation();
    const [isEditingName, setIsEditingName] = createSignal(false);
    const [nameDraft, setNameDraft] = createSignal('');
    const [submittingRename, setSubmittingRename] = createSignal(false);
    const [submittingDelete, setSubmittingDelete] = createSignal(false);
    const [error, setError] = createSignal<string | null>(null);
    createEffect(on(() => [solidProps1.artifact], () => {
        const cleanup = untrack(() => {
            setIsEditingName(false);
            setSubmittingRename(false);
            setSubmittingDelete(false);
            setError(null);
            setNameDraft(solidProps1.artifact?.name || '');
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const handleClose = () => {
        if (submittingRename() || submittingDelete()) {
            return;
        }
        solidProps1.onClose?.();
    };
    const handleRename = async () => {
        if (!solidProps1.artifact || !solidProps1.onRename) {
            return;
        }
        const nextName = nameDraft().trim();
        if (!nextName || nextName === solidProps1.artifact!.name) {
            setIsEditingName(false);
            setNameDraft(solidProps1.artifact!.name);
            return;
        }
        setSubmittingRename(true);
        setError(null);
        try {
            await solidProps1.onRename?.(solidProps1.artifact, nextName);
            setIsEditingName(false);
        }
        catch (err) {
            setError(err instanceof Error ? err.message : solidState2.t('BiChat.Artifacts.RenameFailed'));
        }
        finally {
            setSubmittingRename(false);
        }
    };
    const handleDelete = async () => {
        if (!solidProps1.artifact || !solidProps1.onDelete) {
            return;
        }
        if (!window.confirm(solidState2.t('BiChat.Artifacts.DeleteConfirm'))) {
            return;
        }
        setSubmittingDelete(true);
        setError(null);
        try {
            await solidProps1.onDelete?.(solidProps1.artifact);
            solidProps1.onClose?.();
        }
        catch (err) {
            setError(err instanceof Error ? err.message : solidState2.t('BiChat.Artifacts.DeleteFailed'));
        }
        finally {
            setSubmittingDelete(false);
        }
    };
    return <Show when={!(!solidProps1.artifact)}>{_visible => {
            return (<InlineDialog open={solidProps1.isOpen} onClose={handleClose} className="relative z-50">
      <InlineDialogBackdrop className="fixed inset-0 bg-black/50 backdrop-blur-sm"/>

      <div class="fixed inset-0 overflow-y-auto p-4 lg:p-6">
        <div class="mx-auto flex min-h-full w-full max-w-6xl items-center justify-center">
          <InlineDialogPanel className="flex max-h-[92vh] w-full flex-col overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-xl dark:border-gray-700 dark:bg-gray-900">
            <div class="flex items-start justify-between gap-3 border-b border-gray-200 px-4 py-3 dark:border-gray-700">
              <div class="min-w-0 flex-1">
                {isEditingName() ? (<div class="flex items-center gap-2">
                    <input value={nameDraft()} onInput={(e) => setNameDraft(e.target.value)} onKeyDown={(e) => {
                        if (e.key === 'Enter') {
                            e.preventDefault();
                            void handleRename();
                        }
                        if (e.key === 'Escape') {
                            e.preventDefault();
                            setIsEditingName(false);
                            setNameDraft(solidProps1.artifact!.name);
                        }
                    }} class="w-full rounded-lg border border-gray-300 px-3 py-1.5 text-sm text-gray-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100" aria-label={solidState2.t('BiChat.Artifacts.Rename')} autofocus/>
                    <button type="button" onClick={() => {
                        void handleRename();
                    }} disabled={submittingRename()} class="cursor-pointer inline-flex items-center gap-1 rounded-lg bg-primary-600 px-2.5 py-1.5 text-xs font-medium text-white transition-colors hover:bg-primary-700 disabled:cursor-not-allowed disabled:opacity-60">
                      <FloppyDisk className="h-3.5 w-3.5" weight="bold"/>
                      {solidState2.t('BiChat.Message.Save')}
                    </button>
                    <button type="button" onClick={() => {
                        setIsEditingName(false);
                        setNameDraft(solidProps1.artifact!.name);
                    }} disabled={submittingRename()} class="cursor-pointer rounded-lg border border-gray-200 px-2.5 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-gray-700 dark:text-gray-200 dark:hover:bg-gray-800">
                      {solidState2.t('BiChat.Message.Cancel')}
                    </button>
                  </div>) : (<h2 class="truncate text-base font-semibold text-gray-900 dark:text-gray-100">
                    {solidProps1.artifact!.name}
                  </h2>)}
                {solidProps1.artifact!.description && (<p class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400">
                    {solidProps1.artifact!.description}
                  </p>)}
              </div>

              <div class="flex items-center gap-1.5">
                {solidProps1.canRename && solidProps1.onRename && !isEditingName() && (<button type="button" onClick={() => {
                        setError(null);
                        setIsEditingName(true);
                    }} class="cursor-pointer rounded-lg border border-gray-200 p-2 text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800 dark:hover:text-gray-100" aria-label={solidState2.t('BiChat.Artifacts.Rename')} title={solidState2.t('BiChat.Artifacts.Rename')}>
                    <PencilSimple className="h-4 w-4" weight="regular"/>
                  </button>)}
                {solidProps1.canDelete && solidProps1.onDelete && (<button type="button" onClick={() => {
                        void handleDelete();
                    }} disabled={submittingDelete()} class="cursor-pointer rounded-lg border border-red-200 p-2 text-red-600 transition-colors hover:bg-red-50 hover:text-red-700 disabled:cursor-not-allowed disabled:opacity-60 dark:border-red-900/60 dark:text-red-400 dark:hover:bg-red-950/30 dark:hover:text-red-300" aria-label={solidState2.t('BiChat.Artifacts.Delete')} title={solidState2.t('BiChat.Artifacts.Delete')}>
                    <Trash className="h-4 w-4" weight="regular"/>
                  </button>)}
                <button type="button" onClick={handleClose} class="cursor-pointer rounded-lg border border-gray-200 p-2 text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800 dark:hover:text-gray-100" aria-label={solidState2.t('BiChat.Common.Close')} title={solidState2.t('BiChat.Common.Close')}>
                  <X className="h-4 w-4" weight="bold"/>
                </button>
              </div>
            </div>

            <div class="min-h-0 flex-1 overflow-auto p-4 lg:p-5">
              {error() && (<div class="mb-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/70 dark:bg-red-950/30 dark:text-red-300">
                  {error()}
                </div>)}
              <SessionArtifactPreview artifact={solidProps1.artifact!}/>
            </div>
          </InlineDialogPanel>
        </div>
      </div>
    </InlineDialog>);
        }}</Show>;
}
