import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * SearchInput Component
 * Reusable search input with icon, clear button, and keyboard shortcuts
 */
import { MagnifyingGlass, X } from '../icons';
import { useTranslation } from '../hooks/useTranslation';
export interface SearchInputProps {
    /** Current search value */
    value: string;
    /** Callback when value changes */
    onChange: (value: string) => void;
    /** Placeholder text */
    placeholder?: string;
    /** Auto-focus on mount */
    autofocus?: boolean;
    /** Callback when Enter is pressed */
    onSubmit?: (value: string) => void;
    /** Callback when Escape is pressed */
    onEscape?: () => void;
    /** Additional CSS classes for the container */
    className?: string;
    /** Size variant */
    size?: 'sm' | 'md' | 'lg';
    /** Disable the input */
    disabled?: boolean;
    /** ARIA label for accessibility */
    ariaLabel?: string;
}
const sizeClasses = {
    sm: {
        container: 'py-1.5 pl-8 pr-8 text-xs',
        icon: 14,
        clearBtn: 'p-1',
    },
    md: {
        container: 'py-2.5 pl-10 pr-10 text-sm',
        icon: 16,
        clearBtn: 'p-1.5',
    },
    lg: {
        container: 'py-3 pl-12 pr-12 text-base',
        icon: 18,
        clearBtn: 'p-2',
    },
};
function SearchInput(solidProps1Input: SearchInputProps) {
    const solidProps1 = mergeProps({ autofocus: false, className: '', size: 'md', disabled: false } as const, solidProps1Input);
    const solidState2 = useTranslation();
    const resolvedPlaceholder = createMemo(() => solidProps1.placeholder ?? solidState2.t('BiChat.Common.Search'));
    const resolvedAriaLabel = createMemo(() => solidProps1.ariaLabel ?? solidState2.t('BiChat.Common.Search'));
    const inputRef = { current: null } as {
        current: HTMLInputElement | null;
    };
    const sizes = createMemo(() => sizeClasses[solidProps1.size]);
    createEffect(on(() => [solidProps1.autofocus], () => {
        const cleanup = untrack(() => {
            if (solidProps1.autofocus && inputRef.current) {
                inputRef.current.focus();
            }
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const handleClear = () => {
        solidProps1.onChange?.('');
        inputRef.current?.focus();
    };
    const handleKeyDown = (e: KeyboardEvent & {
        currentTarget: HTMLInputElement;
    }) => {
        if (e.key === 'Enter' && solidProps1.onSubmit) {
            e.preventDefault();
            solidProps1.onSubmit?.(solidProps1.value);
        }
        else if (e.key === 'Escape') {
            e.preventDefault();
            if (solidProps1.value && !solidProps1.onEscape) {
                // Default behavior: clear on Escape if no handler provided
                handleClear();
            }
            else if (solidProps1.onEscape) {
                solidProps1.onEscape?.();
            }
        }
    };
    return (<div class={`relative w-full ${solidProps1.className}`} role="search">
      {/* Search Icon */}
      <span class="absolute inset-y-0 left-3 flex items-center pointer-events-none">
        <MagnifyingGlass size={sizes().icon} weight="bold" className="text-gray-400 dark:text-gray-500" aria-hidden="true"/>
      </span>

      {/* Input Field */}
      <input ref={element => inputRef.current = element} type="search" value={solidProps1.value} onInput={(e) => solidProps1.onChange?.(e.target.value)} onKeyDown={handleKeyDown} placeholder={resolvedPlaceholder()} disabled={solidProps1.disabled} class={`w-full ${sizes().container} bg-gray-50 dark:bg-gray-800/50 border border-gray-200 dark:border-gray-700/50 rounded-xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 focus-visible:border-primary-400 dark:focus-visible:border-primary-600 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed`} aria-label={resolvedAriaLabel()}/>

      {/* Clear Button */}
      {solidProps1.value && !solidProps1.disabled && (<button type="button" onClick={handleClear} class={`cursor-pointer absolute inset-y-0 right-2 flex items-center ${sizes().clearBtn} rounded-lg hover:bg-gray-200 dark:hover:bg-gray-700 transition-all duration-200 text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300`} aria-label={solidState2.t('BiChat.Search.Clear')} title={solidState2.t('BiChat.Search.Clear')}>
          <X size={sizes().icon - 2} weight="bold"/>
        </button>)}
    </div>);
}
const MemoizedSearchInput = SearchInput;
MemoizedSearchInput; /* Solid components are named by their declarations. */
export { MemoizedSearchInput as SearchInput };
export default MemoizedSearchInput;
