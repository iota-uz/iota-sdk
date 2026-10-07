import { createContext, createEffect, createSignal, createUniqueId, onCleanup, onMount, Show, splitProps, useContext, children, type JSX } from 'solid-js';
import { Dynamic, Portal } from 'solid-js/web';
interface MenuState {
    readonly open: boolean;
    readonly focus?: boolean;
    readonly active?: boolean;
    close: () => void;
}
interface MenuContext {
    open: () => boolean;
    setOpen: (open: boolean, restoreFocus?: boolean) => void;
    id: string;
    root?: HTMLElement;
    trigger?: HTMLButtonElement;
    content?: HTMLElement;
}
const Context = createContext<MenuContext>();
function context() {
    const value = useContext(Context);
    if (!value)
        throw new Error('Menu content requires its owning menu');
    return value;
}
type LayerProps = Omit<JSX.HTMLAttributes<HTMLDivElement>, 'children'> & {
    children?: JSX.Element | ((state: MenuState) => JSX.Element);
    className?: string | ((state: MenuState) => string);
    as?: string;
};
export function Menu(props: LayerProps) {
    const [open, setOpen] = createSignal(false);
    const id = createUniqueId();
    const menu: MenuContext = { open, id, setOpen(value, restoreFocus = false) {
            setOpen(value);
            if (!value && restoreFocus)
                menu.trigger?.focus();
        } };
    const state: MenuState = { get open() { return open(); }, close: () => menu.setOpen(false, true) };
    const outside = (event: Event) => {
        const target = event.target as Node;
        if (open() && !menu.root?.contains(target) && !menu.content?.contains(target))
            menu.setOpen(false);
    };
    onMount(() => { document.addEventListener('pointerdown', outside); document.addEventListener('focusin', outside); });
    onCleanup(() => { document.removeEventListener('pointerdown', outside); document.removeEventListener('focusin', outside); });
    return <Context.Provider value={menu}><Dynamic component={props.as ?? 'div'} ref={(node: HTMLElement) => { menu.root = node; }} class={typeof props.className === 'function' ? props.className(state) : props.className ?? props.class}>
  {typeof props.children === 'function' ? props.children(state) : props.children}
 </Dynamic></Context.Provider>;
}
export function MenuButton(props: JSX.ButtonHTMLAttributes<HTMLButtonElement> & {
    className?: string;
    as?: string;
}) {
    const menu = context();
    const [local, native] = splitProps(props, ['className', 'class', 'as', 'onClick', 'onKeyDown', 'children', 'ref']);
    const invoke = (handler: JSX.EventHandlerUnion<HTMLButtonElement, MouseEvent> | undefined, event: MouseEvent & {
        currentTarget: HTMLButtonElement;
        target: Element;
    }) => {
        if (typeof handler === 'function')
            handler(event);
        else if (Array.isArray(handler))
            handler[0](handler[1], event);
    };
    return <button {...native} ref={node => {
            menu.trigger = node;
            if (typeof local.ref === 'function')
                local.ref(node);
        }} type={props.type ?? 'button'} class={local.className ?? local.class} aria-haspopup="menu" aria-controls={menu.id} aria-expanded={menu.open()} onClick={event => { invoke(local.onClick, event); menu.setOpen(!menu.open()); }} onKeyDown={event => {
            if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
                event.preventDefault();
                menu.setOpen(true);
            }
            else if (event.key === 'Escape')
                menu.setOpen(false, true);
        }}>{local.children}</button>;
}
export function MenuItems(props: LayerProps & {
    anchor?: string;
    transition?: boolean;
}) {
    const menu = context();
    const [position, setPosition] = createSignal<JSX.CSSProperties>({});
    let frame = 0;
    const measure = () => {
        const trigger = menu.trigger;
        const content = menu.content;
        if (!trigger || !content)
            return;
        const rect = trigger.getBoundingClientRect();
        const size = content.getBoundingClientRect();
        const above = props.anchor?.includes('top') ?? false;
        let left = props.anchor?.includes('end') ? rect.right - size.width : rect.left;
        let top = above ? rect.top - size.height - 8 : rect.bottom + 8;
        if (top + size.height > window.innerHeight - 8)
            top = Math.max(8, rect.top - size.height - 8);
        left = Math.max(8, Math.min(left, window.innerWidth - size.width - 8));
        setPosition({ position: 'fixed', left: left + 'px', top: Math.max(8, top) + 'px' });
    };
    const keyDown = (event: KeyboardEvent) => {
        const items = Array.from(menu.content?.querySelectorAll<HTMLElement>('[role="menuitem"]:not([disabled])') ?? []);
        const index = items.indexOf(document.activeElement as HTMLElement);
        if (event.key === 'Escape') {
            event.preventDefault();
            menu.setOpen(false, true);
        }
        else if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
            event.preventDefault();
            const next = event.key === 'Home' ? 0 : event.key === 'End' ? items.length - 1 : (index + (event.key === 'ArrowUp' ? -1 : 1) + items.length) % items.length;
            items[next]?.focus();
        }
    };
    createEffect(() => {
        if (!menu.open())
            return;
        frame = requestAnimationFrame(() => { measure(); menu.content?.querySelector<HTMLElement>('[role="menuitem"]:not([disabled]),button:not([disabled]),input:not([disabled])')?.focus(); });
        window.addEventListener('resize', measure);
        document.addEventListener('scroll', measure, true);
        onCleanup(() => { cancelAnimationFrame(frame); window.removeEventListener('resize', measure); document.removeEventListener('scroll', measure, true); });
    });
    return <Show when={menu.open()}><Portal mount={document.getElementById('iota-client-route-portals') ?? document.body}>
  <div ref={node => { menu.content = node; }} id={menu.id} role="menu" class={typeof props.className === 'function' ? props.className({ open: true, close: () => menu.setOpen(false) }) : props.className ?? props.class} style={position()} onKeyDown={keyDown}>
   {typeof props.children === 'function' ? props.children({ open: true, close: () => menu.setOpen(false, true) }) : props.children}
  </div>
 </Portal></Show>;
}
export function MenuItem(props: {
    children?: JSX.Element | ((state: MenuState) => JSX.Element);
    disabled?: boolean;
}) {
    const menu = context();
    const [focused, setFocused] = createSignal(false);
    const state: MenuState = { get open() { return menu.open(); }, get focus() { return focused(); }, get active() { return focused(); }, close: () => menu.setOpen(false, true) };
    const content = children(() => typeof props.children === 'function' ? props.children(state) : props.children);
    createEffect(() => {
        for (const node of content.toArray())
            if (node instanceof HTMLElement) {
                const control = node.matches('button,a') ? node : node.querySelector<HTMLElement>('button,a');
                if (!control)
                    continue;
                control.setAttribute('role', 'menuitem');
                control.tabIndex = -1;
            }
    });
    return <div role="presentation" onFocusIn={() => setFocused(true)} onFocusOut={() => setFocused(false)} onMouseEnter={event => event.currentTarget.querySelector<HTMLElement>('[role="menuitem"]:not([disabled])')?.focus()} onClick={event => {
            const target = (event.target as Element).closest<HTMLElement>('[role="menuitem"]');
            if (!props.disabled && target && !target.hasAttribute('disabled'))
                menu.setOpen(false, true);
        }}>{content()}</div>;
}
export const Popover = Menu;
export const PopoverButton = MenuButton;
export const PopoverPanel = MenuItems;
