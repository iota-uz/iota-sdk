import { splitProps } from 'solid-js';
import { Portal as HostPortal } from '@iota-uz/sdk/solid';
type HTMLAttributes<T> = import('solid-js').JSX.HTMLAttributes<T> & {
    className?: string;
};
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * InlineDialog — portal-free dialog components for Shadow DOM environments.
 *
 * Headless UI Dialog forcibly portals to document.body, escaping Shadow DOM
 * and losing all scoped styles. These components render inline so they stay
 * inside the shadow root and inherit its CSS.
 *
 * API mirrors Headless UI Dialog for minimal migration effort.
 */
// ---------------------------------------------------------------------------
// Context — passes onClose from InlineDialog to descendants
// ---------------------------------------------------------------------------
const DialogContext = createContext<(() => void) | null>(null);
// ---------------------------------------------------------------------------
// InlineDialog
// ---------------------------------------------------------------------------
interface InlineDialogProps {
    open: boolean;
    onClose: () => void;
    className?: string;
    children: JSX.Element;
}
export function InlineDialog(props: InlineDialogProps) {
    return <Show when={props.open}><HostPortal surface="modal" label="Dialog" onEscape={props.onClose}>
      <DialogContext.Provider value={props.onClose}>
        <div class={props.className} onClick={props.onClose} tabIndex={-1}>{props.children}</div>
      </DialogContext.Provider>
    </HostPortal></Show>;
}
// ---------------------------------------------------------------------------
// InlineDialogBackdrop — purely visual overlay
// ---------------------------------------------------------------------------
export function InlineDialogBackdrop(props: HTMLAttributes<HTMLDivElement>) {
    return <div aria-hidden="true" {...props}/>;
}
// ---------------------------------------------------------------------------
// InlineDialogPanel — auto-focus + stops click propagation
// ---------------------------------------------------------------------------
export function InlineDialogPanel(rawProps: HTMLAttributes<HTMLDivElement>) {
    const [nativeProps, rest] = splitProps(rawProps, ["children", "onClick"]);
    // Initial focus (including [data-autofocus]) is owned by InlineDialog's
    // useFocusTrap, so the panel no longer self-focuses — that previously stole
    // focus before the trap could record the trigger to restore to on close.
    return (<div onClick={(e) => {
            e.stopPropagation();
            if (typeof nativeProps.onClick === 'function')
                nativeProps.onClick(e);
            else if (Array.isArray(nativeProps.onClick))
                nativeProps.onClick[0](nativeProps.onClick[1], e);
        }} {...rest}>
      {nativeProps.children}
    </div>);
}
// ---------------------------------------------------------------------------
// InlineDialogTitle / InlineDialogDescription — semantic wrappers
// ---------------------------------------------------------------------------
export function InlineDialogTitle(props: HTMLAttributes<HTMLHeadingElement>) {
    return <h2 {...props}/>;
}
export function InlineDialogDescription(props: HTMLAttributes<HTMLParagraphElement>) {
    return <p {...props}/>;
}
