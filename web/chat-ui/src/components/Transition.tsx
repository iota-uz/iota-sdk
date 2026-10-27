import { createEffect, createSignal, onCleanup, Show, type JSX } from 'solid-js';
export function Transition(props: {
    show: boolean;
    children: JSX.Element;
    enter?: string;
    enterFrom?: string;
    enterTo?: string;
    leave?: string;
    leaveFrom?: string;
    leaveTo?: string;
}) {
    const [visible, setVisible] = createSignal(props.show);
    let wrapper: HTMLDivElement | undefined;
    const tokens = (value?: string) => value?.split(/\s+/).filter(Boolean) ?? [];
    createEffect(() => {
        const show = props.show;
        if (show)
            setVisible(true);
        let frame: number | undefined;
        const timer = window.setTimeout(() => {
            const element = wrapper?.firstElementChild;
            if (!element)
                return;
            const from = tokens(show ? props.enterFrom : props.leaveFrom);
            const to = tokens(show ? props.enterTo : props.leaveTo);
            const transition = tokens(show ? props.enter : props.leave);
            element.classList.remove(...tokens(props.enterFrom), ...tokens(props.enterTo), ...tokens(props.leaveFrom), ...tokens(props.leaveTo), ...tokens(props.enter), ...tokens(props.leave));
            element.classList.add(...transition, ...from);
            frame = requestAnimationFrame(() => { element.classList.remove(...from); element.classList.add(...to); });
        }, 0);
        const leave = show ? undefined : window.setTimeout(() => setVisible(false), 200);
        onCleanup(() => {
            clearTimeout(timer);
            clearTimeout(leave);
            if (frame !== undefined)
                cancelAnimationFrame(frame);
        });
    });
    return <Show when={visible()}><div ref={wrapper} style={{ "display": 'contents' }}>{props.children}</div></Show>;
}
