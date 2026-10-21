/** @jsxImportSource solid-js */
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * useMessageActions Hook
 * Provides copy, regenerate, and edit functionality for messages
 */
export interface UseMessageActionsOptions {
    /** Callback when copy succeeds */
    onCopy?: (content: string) => void;
    /** Callback when copy fails */
    onCopyError?: (error: Error) => void;
    /** Callback when regenerate is triggered */
    onRegenerate?: () => void | Promise<void>;
    /** Callback when edit is triggered */
    onEdit?: (content: string) => void | Promise<void>;
    /** Duration to show "copied" state in ms (default: 2000) */
    copiedDuration?: number;
}
export interface UseMessageActionsReturn {
    /** Whether content was recently copied */
    isCopied: boolean;
    /** Whether regenerate is in progress */
    isRegenerating: boolean;
    /** Whether edit is in progress */
    isEditing: boolean;
    /** Copy content to clipboard */
    copy: (content: string) => Promise<void>;
    /** Trigger regenerate action */
    regenerate: () => Promise<void>;
    /** Trigger edit action */
    edit: (content: string) => Promise<void>;
    /** Reset all states */
    reset: () => void;
}
/**
 * Hook for managing message actions (copy, regenerate, edit)
 *
 * @example
 * ```tsx
 * const actions = useMessageActions({
 *   onRegenerate: () => chatContext.regenerateMessage(messageId),
 *   onEdit: (content) => chatContext.editMessage(messageId, content),
 *   onCopy: () => toast.success('Copied!'),
 * })
 *
 * <button onClick={() => actions.copy(message.content)}>
 *   {actions.isCopied ? 'Copied!' : 'Copy'}
 * </button>
 *
 * <button onClick={actions.regenerate} disabled={actions.isRegenerating}>
 *   {actions.isRegenerating ? 'Regenerating...' : 'Regenerate'}
 * </button>
 * ```
 */
export function useMessageActions(options: UseMessageActionsOptions = {}): UseMessageActionsReturn {
    const { onCopy, onCopyError, onRegenerate, onEdit, copiedDuration = 2000 } = options;
    const [isCopied, setIsCopied] = createSignal(false);
    const [isRegenerating, setIsRegenerating] = createSignal(false);
    const [isEditing, setIsEditing] = createSignal(false);
    const copiedTimeoutRef = { current: null } as {
        current: (ReturnType<typeof setTimeout> | null) | null;
    };
    onMount(() => {
        const cleanup = untrack(() => {
            return () => {
                if (copiedTimeoutRef.current) {
                    clearTimeout(copiedTimeoutRef.current);
                    copiedTimeoutRef.current = null;
                }
            };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    });
    const copy = async (content: string) => {
        try {
            await navigator.clipboard.writeText(content);
            setIsCopied(true);
            onCopy?.(content);
            // Clear existing timeout
            if (copiedTimeoutRef.current) {
                clearTimeout(copiedTimeoutRef.current);
            }
            // Reset copied state after duration
            copiedTimeoutRef.current = setTimeout(() => {
                setIsCopied(false);
                copiedTimeoutRef.current = null;
            }, copiedDuration);
        }
        catch (error) {
            const err = error instanceof Error ? error : new Error('Failed to copy');
            onCopyError?.(err);
            throw err;
        }
    };
    const regenerate = async () => {
        if (!onRegenerate) {
            return;
        }
        if (isRegenerating()) {
            return;
        }
        setIsRegenerating(true);
        try {
            await onRegenerate();
        }
        finally {
            setIsRegenerating(false);
        }
    };
    const edit = async (content: string) => {
        if (!onEdit) {
            return;
        }
        if (isEditing()) {
            return;
        }
        setIsEditing(true);
        try {
            await onEdit(content);
        }
        finally {
            setIsEditing(false);
        }
    };
    const reset = () => {
        setIsCopied(false);
        setIsRegenerating(false);
        setIsEditing(false);
        if (copiedTimeoutRef.current) {
            clearTimeout(copiedTimeoutRef.current);
            copiedTimeoutRef.current = null;
        }
    };
    return {
        get isCopied() {
            return isCopied();
        },
        get isRegenerating() {
            return isRegenerating();
        },
        get isEditing() {
            return isEditing();
        },
        copy,
        regenerate,
        edit,
        reset,
    };
}
