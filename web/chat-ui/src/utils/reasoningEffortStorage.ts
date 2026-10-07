const STORAGE_KEY = 'bichat.reasoningEffort';
export function saveReasoningEffort(effort: string): void {
    if (typeof window === 'undefined') {
        return;
    }
    try {
        window.sessionStorage.setItem(STORAGE_KEY, effort);
    }
    catch {
        // ignore storage errors (quota, privacy mode)
    }
}
export function loadReasoningEffort(): string | null {
    if (typeof window === 'undefined') {
        return null;
    }
    try {
        return window.sessionStorage.getItem(STORAGE_KEY);
    }
    catch {
        return null;
    }
}
export function clearReasoningEffort(): void {
    if (typeof window === 'undefined') {
        return;
    }
    try {
        window.sessionStorage.removeItem(STORAGE_KEY);
    }
    catch {
        // ignore storage errors (quota, privacy mode)
    }
}
