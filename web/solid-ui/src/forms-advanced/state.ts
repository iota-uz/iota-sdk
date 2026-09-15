import { createSignal, type Accessor } from 'solid-js'

export function createControllable<T>(value: Accessor<T | undefined>, defaultValue: T): [Accessor<T>, (next: T) => T] {
  const [internal, setInternal] = createSignal(defaultValue)
  const current = () => value() ?? internal()
  const set = (next: T) => {
    if (value() === undefined) setInternal(() => next)
    return next
  }
  return [current, set]
}
