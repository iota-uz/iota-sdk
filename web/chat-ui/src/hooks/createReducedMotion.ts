import { createSignal, onCleanup } from 'solid-js';
export function createReducedMotion() {
    const media = window.matchMedia('(prefers-reduced-motion: reduce)');
    const [reduced, setReduced] = createSignal(media.matches);
    const update = () => setReduced(media.matches);
    media.addEventListener('change', update);
    onCleanup(() => media.removeEventListener('change', update));
    return reduced;
}
