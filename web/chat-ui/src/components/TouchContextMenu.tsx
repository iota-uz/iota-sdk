type FC<P extends Record<string, any>> = import('solid-js').Component<P>;
type CSSProperties = import('solid-js').JSX.CSSProperties;
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { Portal } from 'solid-js/web';
import { useTranslation } from '../hooks/useTranslation';
export interface ContextMenuItem {
    id: string;
    label: string;
    icon?: JSX.Element;
    onClick: () => void;
    variant?: 'default' | 'danger';
    disabled?: boolean;
}
interface TouchContextMenuProps {
    items: ContextMenuItem[];
    isOpen: boolean;
    onClose: () => void;
    anchorRect: DOMRect | null;
}
export const TouchContextMenu: FC<TouchContextMenuProps> = (solidProps1) => {
    const solidState2 = useTranslation();
    const [focusedIndex, setFocusedIndex] = createSignal(-1);
    const menuRef = { current: null } as {
        current: HTMLDivElement | null;
    };
    const itemRefs = { current: [] } as {
        current: (HTMLButtonElement | null)[];
    };
    const enabledIndices = createMemo(() => solidProps1.items.reduce<number[]>((acc, item, i) => {
        if (!item.disabled) {
            acc.push(i);
        }
        return acc;
    }, []));
    const focusItem = (index: number) => {
        setFocusedIndex(index);
        itemRefs.current[index]?.focus();
    };
    // Auto-focus first enabled item on open
    createEffect(on(() => [solidProps1.isOpen, enabledIndices(), focusItem], () => {
        const cleanup = untrack(() => {
            if (!solidProps1.isOpen) {
                setFocusedIndex(-1);
                return;
            }
            // Small delay to let the menu render
            const timer = requestAnimationFrame(() => {
                if (enabledIndices().length > 0) {
                    focusItem(enabledIndices()[0]);
                }
            });
            return () => cancelAnimationFrame(timer);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // Keyboard navigation
    createEffect(on(() => [solidProps1.isOpen, solidProps1.onClose, focusedIndex(), enabledIndices(), focusItem], () => {
        const cleanup = untrack(() => {
            if (!solidProps1.isOpen) {
                return;
            }
            const handleKeyDown = (e: KeyboardEvent) => {
                const currentEnabledPos = enabledIndices().indexOf(focusedIndex());
                switch (e.key) {
                    case 'Escape':
                        e.preventDefault();
                        solidProps1.onClose?.();
                        break;
                    case 'ArrowDown': {
                        e.preventDefault();
                        const nextPos = currentEnabledPos < enabledIndices().length - 1
                            ? currentEnabledPos + 1
                            : 0;
                        focusItem(enabledIndices()[nextPos]);
                        break;
                    }
                    case 'ArrowUp': {
                        e.preventDefault();
                        const prevPos = currentEnabledPos > 0
                            ? currentEnabledPos - 1
                            : enabledIndices().length - 1;
                        focusItem(enabledIndices()[prevPos]);
                        break;
                    }
                    case 'Home':
                        e.preventDefault();
                        if (enabledIndices().length > 0) {
                            focusItem(enabledIndices()[0]);
                        }
                        break;
                    case 'End':
                        e.preventDefault();
                        if (enabledIndices().length > 0) {
                            focusItem(enabledIndices()[enabledIndices().length - 1]);
                        }
                        break;
                }
            };
            document.addEventListener('keydown', handleKeyDown);
            return () => document.removeEventListener('keydown', handleKeyDown);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    return <Show when={!(!solidProps1.isOpen || !solidProps1.anchorRect)}>{_visible => {
            const anchor = solidProps1.anchorRect!;
            const viewportHeight = window.innerHeight;
            const menuEstimatedHeight = solidProps1.items.length * 44 + 16;
            const spaceBelow = viewportHeight - anchor.bottom;
            const shouldShowAbove = spaceBelow < menuEstimatedHeight && anchor.top > spaceBelow;
            const style: CSSProperties = {
                "position": 'fixed',
                "left": `${anchor.left}px`,
                "width": `${anchor.width}px`,
                "z-index": 9999,
                ...(shouldShowAbove
                    ? { "bottom": `${viewportHeight - anchor.top}px` }
                    : { "top": `${anchor.bottom}px` })
            };
            return <Portal mount={document.body}>{<>
      {solidProps1.isOpen && (<>
          <div class="fixed inset-0 z-[9998]" onClick={solidProps1.onClose}/>

          <div ref={element => menuRef.current = element} role="menu" aria-label={solidState2.t('BiChat.ContextMenu')} style={style} class="rounded-xl shadow-xl backdrop-blur bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 overflow-hidden">
            <div class="py-2">
              {solidProps1.items.map((item, index) => (<button ref={(el) => { itemRefs.current[index] = el; }} role="menuitem" tabIndex={focusedIndex() === index ? 0 : -1} onClick={() => {
                                if (!item.disabled) {
                                    item.onClick();
                                    solidProps1.onClose?.();
                                }
                            }} disabled={item.disabled} class={`
                    w-full flex items-center gap-3 px-4 py-2.5 min-h-[44px]
                    text-left text-sm font-medium transition-colors
                    ${item.disabled
                                ? 'opacity-50 cursor-not-allowed'
                                : 'cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-700 active:bg-gray-200 dark:active:bg-gray-600'}
                    ${item.variant === 'danger'
                                ? 'text-red-600 dark:text-red-400'
                                : 'text-gray-900 dark:text-gray-100'}
                  `}>
                  {item.icon && (<span class="flex-shrink-0 w-5 h-5 flex items-center justify-center">
                      {item.icon}
                    </span>)}
                  <span class="flex-1">{item.label}</span>
                </button>))}
            </div>
          </div>
        </>)}
    </>}</Portal>;
        }}</Show>;
};
