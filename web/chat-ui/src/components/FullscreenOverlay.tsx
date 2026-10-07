import { Portal as HostPortal } from '@iota-uz/sdk/solid';
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * Shadow-DOM-safe fullscreen overlay.
 *
 * Headless UI Dialog portals to document.body which escapes the shadow DOM
 * boundary and loses Tailwind styles. This component stays inline within
 * the shadow tree.
 */
import { X } from '../icons';
interface FullscreenOverlayProps {
    title: string;
    onClose: () => void;
    closeLabel: string;
    children: JSX.Element;
}
export function FullscreenOverlay(props: FullscreenOverlayProps) {
    return <HostPortal surface="modal" label={props.title} onEscape={props.onClose}>
      <div class="fixed inset-0">
        <div class="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={props.onClose} aria-hidden="true"/>
        <div class="absolute inset-4 flex flex-col overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-2xl outline-none dark:border-gray-700 dark:bg-gray-900">
          <button type="button" onClick={props.onClose} class="absolute right-3 top-3 z-10 cursor-pointer rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800" aria-label={props.closeLabel}><X size={18} weight="bold"/></button>
          {props.children}
        </div>
      </div>
    </HostPortal>;
}
