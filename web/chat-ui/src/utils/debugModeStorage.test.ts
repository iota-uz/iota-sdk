import { beforeEach, describe, expect, it } from 'vitest';
import { loadDebugMode, saveDebugMode } from './debugModeStorage';

function createMemorySessionStorage(): Storage {
  const store = new Map<string, string>();
  return {
    get length() {
      return store.size;
    },
    clear() {
      store.clear();
    },
    getItem(key: string) {
      return store.has(key) ? store.get(key)! : null;
    },
    key(index: number) {
      return Array.from(store.keys())[index] ?? null;
    },
    removeItem(key: string) {
      store.delete(key);
    },
    setItem(key: string, value: string) {
      store.set(key, value);
    },
  };
}

describe('debugModeStorage', () => {
  beforeEach(() => {
    Object.defineProperty(globalThis, 'window', {
      value: { sessionStorage: createMemorySessionStorage() },
      configurable: true,
      writable: true,
    });
  });

  it('saves and loads enabled debug mode by session id', () => {
    saveDebugMode('session-1', true);
    expect(loadDebugMode('session-1')).toBe(true);
  });

  it('clears persisted debug mode when disabled', () => {
    saveDebugMode('session-1', true);
    saveDebugMode('session-1', false);
    expect(loadDebugMode('session-1')).toBe(false);
  });

  it('ignores synthetic "new" session id', () => {
    saveDebugMode('new', true);
    expect(loadDebugMode('new')).toBe(false);
  });
});
