import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { X, PencilSimple, Check, ArrowCounterClockwise } from '../icons';
import type { QueuedMessage } from '../types';
import { useTranslation } from '../hooks/useTranslation';
interface MessageQueueListProps {
    queue: QueuedMessage[];
    onRemove: (index: number) => void;
    onUpdate: (index: number, content: string) => void;
}
export function MessageQueueList(solidProps1: MessageQueueListProps) {
    const solidState2 = useTranslation();
    const [editingIndex, setEditingIndex] = createSignal<number | null>(null);
    const [editValue, setEditValue] = createSignal('');
    const startEdit = (index: number) => {
        setEditingIndex(index);
        setEditValue(solidProps1.queue[index].content);
    };
    const saveEdit = () => {
        const index = editingIndex();
        if (index === null) {
            return;
        }
        const trimmed = editValue().trim();
        if (trimmed) {
            solidProps1.onUpdate?.(index, trimmed);
        }
        setEditingIndex(null);
        setEditValue('');
    };
    const cancelEdit = () => {
        setEditingIndex(null);
        setEditValue('');
    };
    return <Show when={!(solidProps1.queue.length === 0)}>{_visible => {
            return (<div class="mb-3 space-y-1.5">
      <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
        <span class="font-medium">{solidState2.t('BiChat.Input.QueuedMessages', { count: solidProps1.queue.length })}</span>
      </div>
      
        {solidProps1.queue.map((item, index) => (<div class="overflow-hidden">
            <div class="flex items-start gap-2 rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 p-2 text-sm">
              <span class="flex-shrink-0 mt-0.5 w-5 h-5 rounded-full bg-primary-100 dark:bg-primary-900/30 text-primary-600 dark:text-primary-400 flex items-center justify-center text-[10px] font-bold">
                {index + 1}
              </span>
              {editingIndex() === index ? (<div class="flex-1 min-w-0 flex flex-col gap-1.5">
                  <textarea value={editValue()} onInput={(e) => setEditValue(e.target.value)} onKeyDown={(e) => {
                            if (e.key === 'Enter' && !e.shiftKey) {
                                e.preventDefault();
                                saveEdit();
                            }
                            if (e.key === 'Escape') {
                                cancelEdit();
                            }
                        }} class="w-full resize-none rounded border border-primary-300 dark:border-primary-600 bg-transparent px-2 py-1 text-sm text-gray-900 dark:text-white outline-none focus:ring-1 focus:ring-primary-500" rows={1} autofocus/>
                  <div class="flex gap-1">
                    <button type="button" onClick={saveEdit} class="cursor-pointer inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium text-primary-700 dark:text-primary-300 bg-primary-50 dark:bg-primary-900/30 hover:bg-primary-100 dark:hover:bg-primary-900/50 rounded transition-colors">
                      <Check size={12} weight="bold"/>
                      {solidState2.t('BiChat.Message.Save')}
                    </button>
                    <button type="button" onClick={cancelEdit} class="cursor-pointer inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors">
                      <ArrowCounterClockwise size={12}/>
                      {solidState2.t('BiChat.Message.Cancel')}
                    </button>
                  </div>
                </div>) : (<p class="flex-1 min-w-0 text-gray-700 dark:text-gray-300 truncate">
                  {item.content}
                  {item.attachments.length > 0 && (<span class="ml-1.5 text-gray-400 dark:text-gray-500">
                      +{item.attachments.length} {solidState2.t('BiChat.Input.AttachFiles').toLowerCase()}
                    </span>)}
                </p>)}
              {editingIndex() !== index && (<div class="flex items-center gap-0.5 flex-shrink-0">
                  <button type="button" onClick={() => startEdit(index)} class="cursor-pointer p-1 text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors" aria-label={solidState2.t('BiChat.Input.EditQueueItem')} title={solidState2.t('BiChat.Input.EditQueueItem')}>
                    <PencilSimple size={14}/>
                  </button>
                  <button type="button" onClick={() => solidProps1.onRemove?.(index)} class="cursor-pointer p-1 text-gray-400 dark:text-gray-500 hover:text-red-500 dark:hover:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors" aria-label={solidState2.t('BiChat.Input.RemoveQueueItem')} title={solidState2.t('BiChat.Input.RemoveQueueItem')}>
                    <X size={14}/>
                  </button>
                </div>)}
            </div>
          </div>))}
      
    </div>);
        }}</Show>;
}
