import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { Lightning, Brain } from '../icons';
import { useChatSession } from '../context/ChatContext';
import { useIotaContext } from '../context/IotaContext';
import { useTranslation } from '../hooks/useTranslation';
interface ModelEntry {
    id: string;
    label: string;
    default?: boolean;
}
export function ModelSelector() {
    const solidState1 = useChatSession();
    const context = useIotaContext();
    const solidState2 = useTranslation();
    const models = createMemo<ModelEntry[]>(() => context.extensions?.llm?.models ?? []);
    const defaultModel = createMemo(() => models().find((m) => m.default) ?? models()[0]);
    const currentModel = createMemo(() => solidState1.model ?? defaultModel()?.id);
    // Set default model on mount
    createEffect(on(() => [solidState1.model, defaultModel(), solidState1.setModel], () => {
        const cleanup = untrack(() => {
            if (!solidState1.model && defaultModel()) {
                solidState1.setModel(defaultModel().id);
            }
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // Keyboard shortcut: Cmd+Shift+M to rotate
    const rotateModel = () => {
        const currentIndex = models().findIndex((m) => m.id === currentModel());
        const nextIndex = (currentIndex + 1) % models().length;
        solidState1.setModel(models()[nextIndex].id);
    };
    createEffect(on(() => [rotateModel], () => {
        const cleanup = untrack(() => {
            const handler = (e: KeyboardEvent) => {
                const isModifierPressed = e.metaKey || e.ctrlKey;
                const isShortcutKey = e.code === 'KeyM' || e.key.toLowerCase() === 'm';
                if (isModifierPressed && e.shiftKey && !e.altKey && isShortcutKey) {
                    e.preventDefault();
                    rotateModel();
                }
            };
            document.addEventListener('keydown', handler);
            return () => document.removeEventListener('keydown', handler);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    return <Show when={!(models().length < 2)}>{_visible => {
            return (<div class="flex items-center justify-between px-4 pt-3 pb-1">
      <div class="inline-flex rounded-lg bg-gray-100 p-0.5 dark:bg-gray-800">
        {models().map((m, i) => {
                    const isActive = createMemo(() => m.id === currentModel());
                    const isFast = i === 0;
                    return (<button type="button" onClick={() => solidState1.setModel(m.id)} class={`
                flex items-center gap-1 rounded-md px-2.5 py-1 text-xs font-medium transition-all duration-150
                ${isActive() ? isFast
                            ? 'bg-white text-amber-600 shadow-sm dark:bg-gray-700 dark:text-amber-400'
                            : 'bg-white text-blue-600 shadow-sm dark:bg-gray-700 dark:text-blue-400'
                            : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'}
              `}>
              {isFast ? <Lightning size={13} weight="fill"/> : <Brain size={13} weight="fill"/>}
              <span>{solidState2.t(m.label)}</span>
            </button>);
                })}
      </div>
      <span class="hidden select-none text-[10px] text-gray-400 sm:block dark:text-gray-500">
        {navigator.platform.includes('Mac') ? '\u2318' : 'Ctrl'}{'\u21E7'}M
      </span>
    </div>);
        }}</Show>;
}
