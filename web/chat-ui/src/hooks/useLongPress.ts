/** @jsxImportSource solid-js */
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
interface LongPressOptions {
    delay?: number; // Default: 500ms
    onLongPress: (e: TouchEvent | MouseEvent) => void;
    onPressStart?: () => void;
    onPressCancel?: () => void;
    moveThreshold?: number; // Default: 10px - cancel if moved more
    hapticFeedback?: boolean; // Default: true
}
interface LongPressEventHandlers {
    onTouchStart: (e: TouchEvent) => void;
    onTouchEnd: (e: TouchEvent) => void;
    onTouchMove: (e: TouchEvent) => void;
    onMouseDown?: (e: MouseEvent) => void; // For desktop testing
    onMouseUp?: (e: MouseEvent) => void;
    onMouseLeave?: (e: MouseEvent) => void;
}
interface LongPressResult {
    handlers: LongPressEventHandlers;
    isPressed: boolean;
}
export function useLongPress(options: LongPressOptions): LongPressResult {
    const { delay = 500, onLongPress, onPressStart, onPressCancel, moveThreshold = 10, hapticFeedback = true, } = options;
    const [isPressed, setIsPressed] = createSignal(false);
    const timerRef = { current: null } as {
        current: (ReturnType<typeof setTimeout> | null) | null;
    };
    const startPosRef = { current: null } as {
        current: ({
            x: number;
            y: number;
        } | null) | null;
    };
    const eventRef = { current: null } as {
        current: (TouchEvent | MouseEvent | null) | null;
    };
    const clearTimer = () => {
        if (timerRef.current) {
            clearTimeout(timerRef.current);
            timerRef.current = null;
        }
    };
    const handlePressStart = (e: TouchEvent | MouseEvent) => {
        // Note: Do NOT call preventDefault() on touchstart - it breaks long-press on iPadOS Safari
        // Text selection is prevented via CSS user-select: none on .touch-tap class instead
        setIsPressed(true);
        eventRef.current = e;
        // Store starting position
        const clientX = 'touches' in e ? e.touches[0].clientX : e.clientX;
        const clientY = 'touches' in e ? e.touches[0].clientY : e.clientY;
        startPosRef.current = { x: clientX, y: clientY };
        onPressStart?.();
        clearTimer();
        timerRef.current = setTimeout(() => {
            if (hapticFeedback && navigator.vibrate) {
                navigator.vibrate(10);
            }
            onLongPress(eventRef.current!);
        }, delay);
    };
    const handlePressEnd = () => {
        clearTimer();
        setIsPressed(false);
        startPosRef.current = null;
        eventRef.current = null;
    };
    const handlePressCancel = () => {
        clearTimer();
        setIsPressed(false);
        startPosRef.current = null;
        eventRef.current = null;
        onPressCancel?.();
    };
    const handleMove = (e: TouchEvent | MouseEvent) => {
        if (!startPosRef.current || !isPressed()) {
            return;
        }
        const clientX = 'touches' in e ? e.touches[0].clientX : e.clientX;
        const clientY = 'touches' in e ? e.touches[0].clientY : e.clientY;
        const deltaX = Math.abs(clientX - startPosRef.current.x);
        const deltaY = Math.abs(clientY - startPosRef.current.y);
        if (deltaX > moveThreshold || deltaY > moveThreshold) {
            handlePressCancel();
        }
    };
    // Cleanup on unmount
    createEffect(on(() => [clearTimer], () => {
        const cleanup = untrack(() => {
            return () => {
                clearTimer();
            };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    return {
        handlers: {
            onTouchStart: handlePressStart,
            onTouchEnd: handlePressEnd,
            onTouchMove: handleMove,
            onMouseDown: handlePressStart,
            onMouseUp: handlePressEnd,
            onMouseLeave: handlePressCancel,
        },
        get isPressed() {
            return isPressed();
        },
    };
}
