import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * EditableText Component
 * Inline editable text with double-click to edit
 * Features: auto-focus, auto-select, Enter to save, Escape to cancel
 * Can be triggered programmatically via ref.startEditing()
 */
import { CircleNotch } from '../icons';
import { useTranslation } from '../hooks/useTranslation';
export interface EditableTextProps {
    /** Current text value */
    value: string;
    /** Callback when text is saved */
    onSave: (newValue: string) => void;
    /** Maximum character length */
    maxLength?: number;
    /** Whether the component is in loading state */
    isLoading?: boolean;
    /** Placeholder text when empty */
    placeholder?: string;
    /** Additional CSS classes for the text display */
    className?: string;
    /** Additional CSS classes for the input */
    inputClassName?: string;
    /** Font size variant */
    size?: 'sm' | 'md' | 'lg';
}
export interface EditableTextRef {
    /** Programmatically start editing mode */
    startEditing: () => void;
    /** Programmatically cancel editing */
    cancelEditing: () => void;
}
const sizeClasses = {
    sm: 'text-sm',
    md: 'text-base',
    lg: 'text-lg',
};
const EditableText = ((solidProps1Input: EditableTextProps & {
    ref?: (handle: EditableTextRef | undefined) => void;
}) => {
    const solidProps1 = mergeProps({ maxLength: 100, isLoading: false, className: '', inputClassName: '', size: 'sm' } as const, solidProps1Input);
    const solidState2 = useTranslation();
    const resolvedPlaceholder = solidProps1.placeholder ?? solidState2.t('BiChat.Common.Untitled');
    const [isEditing, setIsEditing] = createSignal(false);
    const [editValue, setEditValue] = createSignal(solidProps1.value);
    const inputRef = { current: null } as {
        current: HTMLInputElement | null;
    };
    // Expose methods via ref
    const handle = ({
        startEditing: () => {
            setIsEditing(true);
        },
        cancelEditing: () => {
            setEditValue(solidProps1.value);
            setIsEditing(false);
        },
    });
    solidProps1.ref?.(handle);
    onCleanup(() => solidProps1.ref?.(undefined));
    // Update edit value when value prop changes
    createEffect(on(() => [solidProps1.value], () => {
        const cleanup = untrack(() => {
            setEditValue(solidProps1.value);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // Auto-focus and select when entering edit mode
    createEffect(on(() => [isEditing()], () => {
        const cleanup = untrack(() => {
            if (isEditing() && inputRef.current) {
                inputRef.current.focus();
                inputRef.current.select();
            }
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const handleSave = () => {
        const trimmed = editValue().trim();
        // Don't save if empty - revert to original
        if (!trimmed) {
            setEditValue(solidProps1.value);
            setIsEditing(false);
            return;
        }
        // Only call onSave if value actually changed
        if (trimmed !== solidProps1.value) {
            solidProps1.onSave?.(trimmed);
        }
        setIsEditing(false);
    };
    const handleCancel = () => {
        setEditValue(solidProps1.value);
        setIsEditing(false);
    };
    const handleKeyDown = (e: KeyboardEvent) => {
        e.stopPropagation();
        if (e.key === 'Enter') {
            e.preventDefault();
            handleSave();
        }
        else if (e.key === 'Escape') {
            e.preventDefault();
            handleCancel();
        }
    };
    const handleDoubleClick = () => {
        setIsEditing(true);
    };
    const handleBlur = () => {
        handleSave();
    };
    const sizeClass = sizeClasses[solidProps1.size];
    if (isEditing()) {
        return (<div class="flex items-center gap-2 flex-1" onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
            }} onPointerDown={(e) => e.stopPropagation()}>
          <input ref={element => inputRef.current = element} type="text" value={editValue()} onInput={(e) => setEditValue(e.target.value)} onKeyDown={handleKeyDown} onKeyUp={(e) => e.stopPropagation()} onBlur={handleBlur} maxLength={solidProps1.maxLength} placeholder={resolvedPlaceholder} onClick={(e) => e.stopPropagation()} onPointerDown={(e) => e.stopPropagation()} class={`flex-1 px-2 py-1 ${sizeClass} bg-white dark:bg-gray-700 border border-primary-500 dark:border-primary-600 rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 dark:focus-visible:ring-primary-600/30 text-gray-900 dark:text-white ${solidProps1.inputClassName}`} aria-label={solidState2.t('BiChat.EditableText.AriaLabel')}/>
        </div>);
    }
    const displayValue = solidProps1.value || resolvedPlaceholder;
    return (<span onDblClick={handleDoubleClick} class={`${sizeClass} font-medium truncate flex-1 cursor-pointer select-none hover:text-primary-600 dark:hover:text-primary-400 transition-colors ${solidProps1.className}`} title={solidState2.t('BiChat.EditableText.DoubleClickToEdit')} role="button" tabIndex={0} onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                handleDoubleClick();
            }
        }}>
        {solidProps1.isLoading ? (<span class="inline-flex items-center gap-2 text-gray-400 dark:text-gray-500">
            <CircleNotch size={12} className="animate-spin"/>
            <span class="italic">{displayValue}</span>
          </span>) : (<span class={!solidProps1.value ? 'text-gray-400 dark:text-gray-500 italic' : ''}>
            {displayValue}
          </span>)}
      </span>);
});
EditableText; /* Solid components are named by their declarations. */
const MemoizedEditableText = EditableText;
export { MemoizedEditableText as EditableText };
export default MemoizedEditableText;
