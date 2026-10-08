import { createSignal, onCleanup } from 'solid-js';
export interface SwipeInfo {
    offset: {
        x: number;
        y: number;
    };
    velocity: {
        x: number;
        y: number;
    };
}
export function createHorizontalSwipe(options: {
    enabled?: () => boolean;
    left?: number;
    right?: number;
    onStart?: () => void;
    onEnd?: (event: PointerEvent, info: SwipeInfo) => void;
}) {
    const [offset, setOffset] = createSignal(0);
    let active: {
        id: number;
        x: number;
        y: number;
        lastX: number;
        lastAt: number;
        velocity: number;
        element: HTMLElement;
        dragging: boolean;
    } | undefined;
    const finish = (event?: PointerEvent, cancelled = false) => {
        const current = active;
        if (!current || (event && event.pointerId !== current.id))
            return;
        active = undefined;
        if (current.element.hasPointerCapture?.(current.id))
            current.element.releasePointerCapture(current.id);
        if (event && current.dragging && !cancelled)
            options.onEnd?.(event, { offset: { x: event.clientX - current.x, y: event.clientY - current.y }, velocity: { x: current.velocity, y: 0 } });
        setOffset(0);
    };
    onCleanup(() => finish(undefined, true));
    return {
        offset,
        handlers: {
            onPointerDown(event: PointerEvent & {
                currentTarget: HTMLElement;
            }) {
                if (options.enabled?.() === false || event.button !== 0 || (event.target as Element)?.closest('button,a,input,textarea,select'))
                    return;
                active = { id: event.pointerId, x: event.clientX, y: event.clientY, lastX: event.clientX, lastAt: event.timeStamp, velocity: 0, element: event.currentTarget, dragging: false };
            },
            onPointerMove(event: PointerEvent) {
                const current = active;
                if (!current || event.pointerId !== current.id)
                    return;
                const x = event.clientX - current.x;
                const y = event.clientY - current.y;
                if (!current.dragging && Math.abs(y) > Math.abs(x) + 5) {
                    finish(event, true);
                    return;
                }
                if (!current.dragging && Math.abs(x) > 5) {
                    current.dragging = true;
                    current.element.setPointerCapture(event.pointerId);
                    options.onStart?.();
                }
                if (!current.dragging)
                    return;
                event.preventDefault();
                current.velocity = (event.clientX - current.lastX) / Math.max(1, event.timeStamp - current.lastAt) * 1000;
                current.lastX = event.clientX;
                current.lastAt = event.timeStamp;
                setOffset(Math.max(options.left ?? -120, Math.min(options.right ?? 0, x)));
            },
            onPointerUp(event: PointerEvent) { finish(event); },
            onPointerCancel(event: PointerEvent) { finish(event, true); },
        },
    };
}
