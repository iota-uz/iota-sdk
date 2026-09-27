import { createSignal, For, Show, splitProps, type JSX } from 'solid-js'
import { Avatar } from '../data/Avatar'
import { classes } from '../internal/classes'

export interface SidebarLinkNode { kind?: 'link'; id?: string; text: string; href: string; icon?: JSX.Element; beta?: boolean; active?: boolean }
export interface SidebarGroupNode { kind: 'group'; id: string; text: string; icon?: JSX.Element; beta?: boolean; active?: boolean; children: readonly SidebarNode[] }
export type SidebarNode = SidebarLinkNode | SidebarGroupNode
export interface SidebarWorkspace { value: string; label: string; beta?: boolean; nodes: readonly SidebarNode[] }

function Caret(props: { direction?: 'down' | 'left' | 'right' }) {
  const points = () => props.direction === 'left' ? '160 208 80 128 160 48' : props.direction === 'right' ? '96 48 176 128 96 208' : '208 96 128 176 48 96'
  return <svg aria-hidden="true" width="16" height="16" viewBox="0 0 256 256"><polyline points={points()} fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" /></svg>
}

export function SidebarBetaBadge(props: { active?: boolean; class?: string }) { return <span class={classes('text-[10px] px-2 py-0.5 rounded-full leading-none', props.active ? 'bg-blue-100 text-blue-700' : 'bg-surface-300 text-200', props.class)}>Beta</span> }

export function SidebarFallbackIcon(props: { text: string }) {
  const initial = () => Array.from(props.text.trim())[0]?.toUpperCase() ?? '?'
  return <span aria-hidden="true" class="w-6 h-6 rounded-full border border-current/40 text-[11px] font-semibold leading-none flex items-center justify-center shrink-0">{initial()}</span>
}

export interface SidebarNavigationProps extends JSX.HTMLAttributes<HTMLElement> { nodes: readonly SidebarNode[]; collapsed?: boolean; onNavigate?: (node: SidebarLinkNode, event: MouseEvent) => void }

function NavigationNode(props: { node: SidebarNode; collapsed: boolean; depth: number; onNavigate?: SidebarNavigationProps['onNavigate'] }) {
  const [flyout, setFlyout] = createSignal(false)
  const [flyoutStyle, setFlyoutStyle] = createSignal<JSX.CSSProperties>()
  const tooltip = () => `${props.node.text}${props.node.beta ? ' (Beta)' : ''}`
  if (props.node.kind !== 'group') return <li>{props.collapsed
    ? <a href={props.node.href} aria-label={tooltip()} class={classes('shrink-0 btn cursor-pointer btn-sidebar btn-md p-2 w-auto flex justify-center', props.node.active && 'active')} onClick={(event) => props.onNavigate?.(props.node as SidebarLinkNode, event)}><div class="w-6 h-6 flex items-center justify-center">{props.node.icon ?? <SidebarFallbackIcon text={props.node.text} />}</div></a>
    : <a href={props.node.href} class={classes('shrink-0 btn cursor-pointer btn-sidebar btn-md gap-2 w-full', props.node.active && 'active')} onClick={(event) => props.onNavigate?.(props.node as SidebarLinkNode, event)}>{props.node.icon}{props.node.text}{props.node.beta && <SidebarBetaBadge active={props.node.active} class="ml-auto" />}<div class="btn-loading-indicator" /></a>}
  </li>
  return <li class="relative">{props.collapsed
    ? <><button type="button" aria-label={tooltip()} aria-expanded={flyout()} data-sidebar-collapsed-group-trigger="true" data-group-id={props.node.id} data-depth={props.depth} class="accordion-group-collapsed w-full cursor-pointer" onClick={(event) => { const rect = event.currentTarget.getBoundingClientRect(); setFlyoutStyle({ left: `${rect.right + 8}px`, top: `${rect.top}px` }); setFlyout(!flyout()) }} onKeyDown={(event) => { if (event.key === 'Escape') setFlyout(false) }}>{props.node.icon ?? <SidebarFallbackIcon text={props.node.text} />}</button>{flyout() && <div data-sidebar-collapsed-menu="true" data-group-id={props.node.id} data-depth={props.depth} class="sidebar-flyout fixed z-[70] w-64 rounded-lg p-1.5" style={flyoutStyle()} onKeyDown={(event) => { if (event.key === 'Escape') setFlyout(false) }}><ul class="flex flex-col gap-1"><For each={props.node.children}>{(node) => <NavigationNode node={node} collapsed={false} depth={props.depth + 1} onNavigate={props.onNavigate} />}</For></ul></div>}</>
    : <details class="group" open={props.node.active}><summary class="btn btn-sidebar btn-md gap-2 w-full cursor-pointer">{props.node.icon}{props.node.text}{props.node.beta && <SidebarBetaBadge active={props.node.active} class="ml-auto" />}<span class={classes('ml-auto duration-200 group-open:rotate-180', props.node.beta && 'ml-0')}><Caret /></span></summary><ul class="ml-4 mt-2 flex flex-col gap-2"><For each={props.node.children}>{(node) => <NavigationNode node={node} collapsed={false} depth={props.depth + 1} onNavigate={props.onNavigate} />}</For></ul></details>}
  </li>
}

export function SidebarNavigation(props: SidebarNavigationProps) {
  const [local, native] = splitProps(props, ['nodes', 'collapsed', 'onNavigate', 'class'])
  return <nav {...native} class={classes('py-4 flex-1 min-h-0 overflow-y-auto hide-scrollbar', local.class)}><Show when={local.collapsed} fallback={<ul class="flex flex-col gap-2 transition-all duration-300 px-6"><For each={local.nodes}>{(node) => <NavigationNode node={node} collapsed={false} depth={0} onNavigate={local.onNavigate} />}</For></ul>}><ul class="flex flex-col gap-2 transition-all duration-300 px-2"><For each={local.nodes}>{(node) => <NavigationNode node={node} collapsed depth={0} onNavigate={local.onNavigate} />}</For></ul></Show></nav>
}

export interface SidebarGroupProps extends JSX.HTMLAttributes<HTMLUListElement> { label?: JSX.Element }
export function SidebarGroup(props: SidebarGroupProps) { const [local, native] = splitProps(props, ['label', 'class', 'children']); return <div>{local.label && <div class="px-2.5 pt-2 pb-1 text-xs uppercase tracking-wide text-200">{local.label}</div>}<ul {...native} class={classes('flex flex-col gap-2', local.class)}>{local.children}</ul></div> }

export interface SidebarFooterProps extends JSX.HTMLAttributes<HTMLDivElement> { collapsed?: boolean; onToggle?: () => void }
export function SidebarFooter(props: SidebarFooterProps) { const [local, native] = splitProps(props, ['collapsed', 'onToggle', 'class', 'children']); return <div {...native} class={classes('mt-auto transition-all duration-300', local.collapsed ? 'px-2' : 'px-6', local.class)}><button type="button" class="hidden lg:flex items-center justify-center w-full p-2 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded-md transition-colors" aria-label={local.collapsed ? 'Expand sidebar' : 'Collapse sidebar'} onClick={() => local.onToggle?.()}><Caret direction={local.collapsed ? 'right' : 'left'} /></button>{local.children}</div> }

export interface SidebarUserFooterProps extends Omit<JSX.ButtonHTMLAttributes<HTMLButtonElement>, 'name'> { name: JSX.Element; subtitle?: JSX.Element; initials: string; imageUrl?: string; collapsed?: boolean; onSelect?: () => void }
export function SidebarUserFooter(props: SidebarUserFooterProps) { const [local, native] = splitProps(props, ['name', 'subtitle', 'initials', 'imageUrl', 'collapsed', 'onSelect', 'class', 'onClick']); return <button {...native} type="button" class={classes('flex w-full items-center gap-3 rounded-lg p-2 text-left hover:bg-surface-300 transition-colors', local.collapsed && 'justify-center', local.class)} aria-label={local.collapsed && typeof local.name === 'string' ? local.name : undefined} onClick={(event) => { local.onSelect?.(); if (typeof local.onClick === 'function') local.onClick(event) }}><Avatar initials={local.initials} imageUrl={local.imageUrl} class="shrink-0" /><Show when={!local.collapsed}><span class="min-w-0"><span class="block truncate text-sm font-medium text-100">{local.name}</span>{local.subtitle && <span class="block truncate text-xs text-300">{local.subtitle}</span>}</span></Show></button> }

export interface SidebarProps extends JSX.HTMLAttributes<HTMLDivElement> { header?: JSX.Element; workspaces: readonly SidebarWorkspace[]; pinned?: readonly SidebarNode[]; footer?: JSX.Element; collapsed?: boolean; defaultCollapsed?: boolean; onCollapsedChange?: (collapsed: boolean) => void; workspace?: string; defaultWorkspace?: string; onWorkspaceChange?: (value: string) => void; onNavigate?: SidebarNavigationProps['onNavigate'] }

export function Sidebar(props: SidebarProps) {
  const [local, native] = splitProps(props, ['header', 'workspaces', 'pinned', 'footer', 'collapsed', 'defaultCollapsed', 'onCollapsedChange', 'workspace', 'defaultWorkspace', 'onWorkspaceChange', 'onNavigate', 'class'])
  const [internalCollapsed, setInternalCollapsed] = createSignal(local.defaultCollapsed ?? false)
  const [internalWorkspace, setInternalWorkspace] = createSignal(local.defaultWorkspace ?? local.workspaces[0]?.value ?? '')
  const collapsed = () => local.collapsed ?? internalCollapsed()
  const activeWorkspace = () => local.workspace ?? internalWorkspace()
  const setCollapsed = (value: boolean) => { if (local.collapsed === undefined) setInternalCollapsed(value); local.onCollapsedChange?.(value) }
  const setWorkspace = (value: string) => { if (local.workspace === undefined) setInternalWorkspace(value); local.onWorkspaceChange?.(value) }
  const workspace = () => local.workspaces.find((item) => item.value === activeWorkspace()) ?? local.workspaces[0]
  return <div {...native} class={classes('flex flex-col bg-surface-200 shadow-lg py-6 h-screen sdk-h-dvh sticky top-0 transition-all duration-300 overflow-visible', collapsed() ? 'sidebar-collapsed' : 'sidebar-expanded', local.class)}>
    <div class={classes('mb-4 transition-all duration-300', collapsed() ? 'px-2' : 'px-6')}>{local.header}{local.pinned && local.pinned.length > 0 && <nav class="mt-4"><ul class="flex flex-col gap-2 pb-4 border-b border-surface-300"><For each={local.pinned}>{(node) => <NavigationNode node={node} collapsed={collapsed()} depth={0} onNavigate={local.onNavigate} />}</For></ul></nav>}{local.workspaces.length > 1 && (collapsed() ? <div role="tablist" aria-orientation="vertical" class="mt-4 flex flex-col items-center gap-1"><For each={local.workspaces}>{(item) => <button type="button" role="tab" aria-label={item.label} aria-selected={activeWorkspace() === item.value} class={classes('relative w-10 h-10 rounded-xl text-[11px] font-semibold leading-none flex items-center justify-center transition-colors duration-200 !cursor-pointer', activeWorkspace() === item.value ? 'bg-white text-slate-900' : 'text-gray-100 hover:bg-white/10')} onClick={() => setWorkspace(item.value)}>{Array.from(item.label.trim()).slice(0, 3).join('').toUpperCase()}{item.beta && <SidebarBetaBadge class="absolute -top-1 -right-1 z-10 pointer-events-none text-[8px] px-1" />}</button>}</For></div> : <div role="tablist" class="mt-4 relative bg-slate-800 rounded-2xl p-1.5 flex items-center gap-1"><For each={local.workspaces}>{(item) => <button type="button" role="tab" aria-selected={activeWorkspace() === item.value} class={classes('relative z-10 py-2 px-3 text-sm font-medium rounded-xl transition-colors duration-200 flex-1 text-center !cursor-pointer', activeWorkspace() === item.value ? 'bg-white text-slate-900' : 'text-gray-500 hover:text-slate-300')} onClick={() => setWorkspace(item.value)}>{item.label}</button>}</For></div>)}</div>
    <Show when={workspace()}>{(item) => <SidebarNavigation nodes={item().nodes} collapsed={collapsed()} onNavigate={local.onNavigate} />}</Show>
    {local.footer ?? <SidebarFooter collapsed={collapsed()} onToggle={() => setCollapsed(!collapsed())} />}
  </div>
}
