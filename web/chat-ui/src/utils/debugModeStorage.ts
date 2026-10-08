const STORAGE_PREFIX = 'bichat.debug.';
function key(sessionId: string): string {
    return `${STORAGE_PREFIX}${sessionId}`;
}
function isPersistableSessionId(sessionId: string): boolean {
    return Boolean(sessionId && sessionId !== 'new');
}
export function saveDebugMode(sessionId: string, enabled: boolean): void {
    if (typeof window === 'undefined') {
        return;
    }
    if (!isPersistableSessionId(sessionId)) {
        return;
    }
    try {
        if (enabled) {
            window.sessionStorage.setItem(key(sessionId), '1');
            return;
        }
        window.sessionStorage.removeItem(key(sessionId));
    }
    catch {
        // ignore storage errors (quota, privacy mode)
    }
}
export function loadDebugMode(sessionId: string): boolean {
    if (typeof window === 'undefined') {
        return false;
    }
    if (!isPersistableSessionId(sessionId)) {
        return false;
    }
    try {
        return window.sessionStorage.getItem(key(sessionId)) === '1';
    }
    catch {
        return false;
    }
}
