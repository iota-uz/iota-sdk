import { createContext, createSignal, onCleanup, onMount, Show, useContext, type Accessor, type JSX } from 'solid-js'

export interface TaggedSlotChunk {
  name: string
  chunk: JSX.Element
}

export interface SlotManager {
  value(name: string): Accessor<JSX.Element | undefined>
  push(name: string, chunk: JSX.Element): void
  clear(name?: string): void
}

export function createSlotManager(): SlotManager {
  const slots = new Map<string, ReturnType<typeof createSignal<JSX.Element | undefined>>>()
  const entry = (name: string) => {
    let current = slots.get(name)
    if (!current) {
      current = createSignal<JSX.Element | undefined>()
      slots.set(name, current)
    }
    return current
  }
  return {
    value: (name) => entry(name)[0],
    push: (name, chunk) => entry(name)[1](() => chunk),
    clear: (name) => {
      if (name) entry(name)[1](undefined)
      else for (const [, setValue] of slots.values()) setValue(undefined)
    },
  }
}

const SlotManagerContext = createContext<SlotManager>()

export interface SlotManagerProviderProps {
  manager: SlotManager
  children: JSX.Element
}

export function SlotManagerProvider(props: SlotManagerProviderProps) {
  return <SlotManagerContext.Provider value={props.manager}>{props.children}</SlotManagerContext.Provider>
}

export interface SlotProps {
  name: string
  fallback?: JSX.Element
  manager?: SlotManager
  empty?: boolean
}

export function Slot(props: SlotProps) {
  const contextual = useContext(SlotManagerContext)
  const manager = () => props.manager ?? contextual
  const content = () => manager()?.value(props.name)() ?? props.fallback
  return <Show when={!props.empty}><div class="contents"><template shadowrootmode="open" innerHTML={`<slot name="${props.name.replace(/[&<>"']/g, '')}"></slot>`} /><div id={`${props.name}-target`} slot={props.name}>{content()}</div></div></Show>
}

export interface StreamerProps {
  manager?: SlotManager
  stream: AsyncIterable<TaggedSlotChunk> | ((signal: AbortSignal) => AsyncIterable<TaggedSlotChunk>)
  onError?: (error: unknown) => void
}

export function Streamer(props: StreamerProps) {
  const contextual = useContext(SlotManagerContext)
  onMount(() => {
    const controller = new AbortController()
    const consume = async () => {
      try {
        const stream = typeof props.stream === 'function' ? props.stream(controller.signal) : props.stream
        for await (const tagged of stream) {
          if (controller.signal.aborted) break
          ;(props.manager ?? contextual)?.push(tagged.name, tagged.chunk)
        }
      } catch (error) {
        if (!controller.signal.aborted) props.onError?.(error)
      }
    }
    void consume()
    onCleanup(() => controller.abort())
  })
  return null
}
