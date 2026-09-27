import { createContext, createEffect, createSignal, createUniqueId, onCleanup, onMount, splitProps, useContext, type Accessor, type JSX } from 'solid-js'
import { classes } from '../internal/classes'

interface NavTabsContextValue {
  active: Accessor<string>
  select(value: string): void
  register(value: string, element: HTMLButtonElement): void
  unregister(value: string): void
  background: Accessor<JSX.CSSProperties>
  id: string
}

const NavTabsContext = createContext<NavTabsContextValue>()

function useNavTabs(): NavTabsContextValue {
  const context = useContext(NavTabsContext)
  if (!context) throw new Error('NavTabs components must be rendered inside NavTabs')
  return context
}

export interface NavTabsProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'onChange'> {
  value?: string
  defaultValue?: string
  onValueChange?: (value: string) => void
}

export function NavTabs(props: NavTabsProps) {
  const [local, native] = splitProps(props, ['value', 'defaultValue', 'onValueChange', 'children'])
  const [internal, setInternal] = createSignal(local.defaultValue ?? local.value ?? '')
  const [background, setBackground] = createSignal<JSX.CSSProperties>({ left: '0px', width: '0px', opacity: 0 })
  const buttons = new Map<string, HTMLButtonElement>()
  let observer: ResizeObserver | undefined
  const active = () => local.value ?? internal()
  const update = () => {
    const button = buttons.get(active())
    setBackground(button ? { left: `${button.offsetLeft}px`, width: `${button.offsetWidth}px`, opacity: 1 } : { left: '0px', width: '0px', opacity: 0 })
  }
  const context: NavTabsContextValue = {
    active,
    select(value) {
      if (local.value === undefined) setInternal(value)
      local.onValueChange?.(value)
    },
    register(value, element) { buttons.set(value, element); observer?.observe(element); update() },
    unregister(value) { const element = buttons.get(value); if (element) observer?.unobserve(element); buttons.delete(value) },
    background,
    id: createUniqueId(),
  }
  createEffect(update)
  onMount(() => {
    const ownerWindow = Array.from(buttons.values())[0]?.ownerDocument.defaultView
    ownerWindow?.addEventListener('resize', update)
    if (typeof ResizeObserver !== 'undefined') {
      observer = new ResizeObserver(update)
      for (const button of buttons.values()) observer.observe(button)
    }
    onCleanup(() => {
      ownerWindow?.removeEventListener('resize', update)
      observer?.disconnect()
    })
  })
  return <NavTabsContext.Provider value={context}><div {...native}>{local.children}</div></NavTabsContext.Provider>
}

export function NavTabsList(props: JSX.HTMLAttributes<HTMLDivElement>) {
  const context = useNavTabs()
  const [local, native] = splitProps(props, ['class', 'children'])
  return (
    <div
      {...native}
      role="tablist"
      aria-label={native['aria-label'] ?? 'Tab navigation'}
      class={classes('relative bg-slate-800 rounded-2xl p-1.5 flex items-center gap-1', local.class)}
      onKeyDown={(event) => {
        if (!['ArrowRight', 'ArrowLeft', 'Home', 'End'].includes(event.key)) return
        const tabs = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>('[role="tab"]:not([disabled])'))
        const current = tabs.indexOf(event.target as HTMLButtonElement)
        if (current < 0 || tabs.length === 0) return
        event.preventDefault()
        const next = event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 : (current + (event.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length
        tabs[next]?.focus()
        tabs[next]?.click()
      }}
    >
      <div aria-hidden="true" class="absolute bg-white rounded-xl top-1.5 h-[calc(100%-0.75rem)] transition-all duration-300 ease-out border-0" style={context.background()} />
      {local.children}
    </div>
  )
}

export interface NavTabsTriggerProps extends Omit<JSX.ButtonHTMLAttributes<HTMLButtonElement>, 'value'> { value: string }

export function NavTabsTrigger(props: NavTabsTriggerProps) {
  const context = useNavTabs()
  const [local, native] = splitProps(props, ['value', 'class', 'children', 'ref', 'onClick'])
  let element!: HTMLButtonElement
  const active = () => context.active() === local.value
  onMount(() => context.register(local.value, element))
  onCleanup(() => context.unregister(local.value))
  return (
    <button
      {...native}
      ref={(node) => { element = node; if (typeof local.ref === 'function') local.ref(node) }}
      id={`${context.id}-tab-${local.value}`}
      type="button"
      role="tab"
      data-tab-value={local.value}
      aria-selected={active()}
      aria-controls={`${context.id}-panel-${local.value}`}
      tabIndex={active() ? 0 : -1}
      class={classes('relative z-10 py-2 px-3 text-sm font-medium rounded-xl transition-colors duration-200 flex-1 text-center !cursor-pointer', active() ? 'text-slate-900' : 'text-gray-500 hover:text-slate-300', local.class)}
      onClick={(event) => { context.select(local.value); if (typeof local.onClick === 'function') local.onClick(event) }}
    >{local.children}</button>
  )
}

export interface NavTabsContentProps extends JSX.HTMLAttributes<HTMLDivElement> { value: string }

export function NavTabsContent(props: NavTabsContentProps) {
  const context = useNavTabs()
  const [local, native] = splitProps(props, ['value', 'children'])
  return <div {...native} id={`${context.id}-panel-${local.value}`} role="tabpanel" aria-labelledby={`${context.id}-tab-${local.value}`} hidden={context.active() !== local.value}>{local.children}</div>
}
