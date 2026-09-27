export function callHandler<TEvent>(handler: unknown, event: TEvent): void {
  if (typeof handler === 'function') handler(event)
}

