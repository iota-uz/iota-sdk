import { createSignal, onCleanup, type Accessor } from 'solid-js';
export function createExternalSnapshot<T>(subscribe: (listener: () => void) => () => void, snapshot: () => T): Accessor<T> {
    const [value, setValue] = createSignal(snapshot(), { equals: false });
    const unsubscribe = subscribe(() => setValue(() => snapshot()));
    // Close the read/subscribe gap for stores that changed during subscription.
    setValue(() => snapshot());
    onCleanup(unsubscribe);
    return value;
}
export function createReactiveSnapshot<T extends object>(subscribe: (listener: () => void) => () => void, snapshot: () => T): T {
    const value = createExternalSnapshot(subscribe, snapshot);
    return new Proxy({} as T, {
        get: (_target, key) => Reflect.get(value(), key),
        has: (_target, key) => Reflect.has(value(), key),
        ownKeys: () => Reflect.ownKeys(value()),
        getOwnPropertyDescriptor: (_target, key) => {
            const descriptor = Reflect.getOwnPropertyDescriptor(value(), key);
            return descriptor && { ...descriptor, configurable: true };
        },
    });
}
export function createReactiveValue<T extends object>(value: Accessor<T>): T {
    return new Proxy({} as T, {
        get: (_target, key) => Reflect.get(value(), key),
        has: (_target, key) => Reflect.has(value(), key),
        ownKeys: () => Reflect.ownKeys(value()),
        getOwnPropertyDescriptor: (_target, key) => {
            const descriptor = Reflect.getOwnPropertyDescriptor(value(), key);
            return descriptor && { ...descriptor, configurable: true };
        },
    });
}
