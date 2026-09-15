import { createSignal } from 'solid-js'
import { fireEvent, render, screen, within } from '@solidjs/testing-library'
import { describe, expect, it, vi } from 'vitest'
import { CommandEmpty, CommandGroup, CommandResult, CommandSearch } from './CommandSearch'
import { Kanban, KanbanBoard, KanbanCard, KanbanColumn } from './Kanban'
import { PermissionGuard } from './PermissionGuard'
import { Sidebar, SidebarFallbackIcon, SidebarGroup, SidebarUserFooter, type SidebarNode } from './Sidebar'

describe('shell and workflow primitives', () => {
  it('renders expanded and collapsed sidebar navigation and changes workspaces', async () => {
    const sales: SidebarNode[] = [{ text: 'Dashboard', href: '/dashboard', active: true }, { kind: 'group', id: 'reports', text: 'Reports', beta: true, active: true, children: [{ text: 'Monthly', href: '/reports/monthly' }] }]
    const workspaceChanged = vi.fn()
    render(() => <Sidebar header={<strong>Logo</strong>} pinned={[{ text: 'Home', href: '/' }]} workspaces={[{ value: 'sales', label: 'Sales', nodes: sales }, { value: 'crm', label: 'CRM', nodes: [{ text: 'Clients', href: '/clients' }] }]} onWorkspaceChange={workspaceChanged} />)
    const sidebar = screen.getByText('Logo').closest('.sidebar-expanded')!
    expect(sidebar).toHaveClass('bg-surface-200', 'shadow-lg', 'sdk-h-dvh', 'sticky')
    expect(screen.getByRole('link', { name: 'Dashboard' })).toHaveClass('btn-sidebar', 'active', 'gap-2', 'w-full')
    expect(screen.getByText('Reports').closest('details')).toHaveAttribute('open')
    await fireEvent.click(screen.getByRole('tab', { name: 'CRM' }))
    expect(workspaceChanged).toHaveBeenCalledWith('crm')
    expect(screen.getByRole('link', { name: 'Clients' })).toBeVisible()
    await fireEvent.click(screen.getByRole('button', { name: 'Collapse sidebar' }))
    expect(screen.getByText('Logo').closest('.sidebar-collapsed')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Clients' })).toHaveClass('p-2', 'w-auto', 'justify-center')
    expect(screen.getByRole('tab', { name: 'Sales' })).toHaveTextContent('SAL')
  })

  it('supports collapsed group flyouts, fallback initials and user footer states', async () => {
    render(() => <><Sidebar workspaces={[{ value: 'erp', label: 'ERP', nodes: [{ kind: 'group', id: 'stock', text: 'Склад', children: [{ text: 'Orders', href: '/orders' }] }] }]} defaultCollapsed /><SidebarFallbackIcon text="  alpha" /><SidebarGroup label="Tools"><li>Item</li></SidebarGroup><SidebarUserFooter name="Dilshod" subtitle="Admin" initials="DK" collapsed /></>)
    const group = screen.getByRole('button', { name: 'Склад' })
    expect(group).toHaveAttribute('data-sidebar-collapsed-group-trigger', 'true')
    await fireEvent.click(group)
    const flyout = document.querySelector('[data-sidebar-collapsed-menu="true"]')!
    expect(flyout).toHaveClass('sidebar-flyout', 'fixed', 'z-[70]', 'w-64')
    expect(within(flyout as HTMLElement).getByRole('link', { name: 'Orders' })).toBeVisible()
    expect(screen.getByText('A')).toHaveClass('rounded-full', 'border-current/40')
    expect(screen.getByRole('button', { name: 'Dilshod' })).not.toHaveTextContent('Admin')
    expect(screen.getByText('Tools')).toHaveClass('uppercase', 'tracking-wide')
  })

  it('renders canonical kanban primitives and exposes keyboard reorder/activation callbacks', async () => {
    const columnMove = vi.fn()
    const activate = vi.fn()
    render(() => <Kanban columns={[{ key: 'todo', title: 'Todo', cards: [{ key: 'one', content: <article>First</article> }] }, { key: 'done', title: 'Done', cards: [] }]} label="Pipeline" onColumnMove={columnMove} onCardActivate={activate} />)
    const board = screen.getByRole('list', { name: 'Pipeline' })
    expect(board).toHaveClass('flex', 'w-max', 'min-w-full', 'divide-x')
    const todo = screen.getByText('Todo').closest('li')!
    expect(todo).toHaveClass('w-72', 'shrink-0', 'p-3')
    await fireEvent.keyDown(todo, { key: 'ArrowRight', altKey: true })
    expect(columnMove).toHaveBeenCalledWith({ key: 'todo', oldIndex: 0, newIndex: 1 })
    const card = screen.getByText('First').closest('[data-card-key]')!
    await fireEvent.keyDown(card, { key: 'Enter' })
    expect(activate).toHaveBeenCalledWith('one', expect.any(KeyboardEvent))
  })

  it('allows standalone kanban board, column and card composition', () => {
    render(() => <KanbanBoard label="Tasks"><KanbanColumn columnKey="open" title="Open"><KanbanCard cardKey="A">Task A</KanbanCard></KanbanColumn></KanbanBoard>)
    expect(screen.getByRole('list', { name: 'Tasks' })).toContainElement(screen.getByText('Task A'))
    expect(screen.getByRole('list', { name: 'Open cards' })).toHaveClass('min-h-6', 'flex-col', 'gap-2')
  })

  it('evaluates all/any permission predicates reactively and renders fallback without wrappers', async () => {
    const [permissions, setPermissions] = createSignal(new Set(['read']))
    const view = render(() => <div data-testid="host"><PermissionGuard permissions={['read', 'write']} can={(permission) => permissions().has(permission)} fallback={<span>Denied</span>}><button>Allowed</button></PermissionGuard></div>)
    expect(screen.getByText('Denied').parentElement).toBe(screen.getByTestId('host'))
    setPermissions(new Set(['read', 'write']))
    expect(await screen.findByRole('button', { name: 'Allowed' })).toBeVisible()
    view.unmount()
    render(() => <PermissionGuard permissions={['admin', 'read']} mode="any" can={(permission) => permission === 'read'} fallback="No">Yes</PermissionGuard>)
    expect(screen.getByText('Yes')).toBeInTheDocument()
  })

  it('opens command search by shortcut, navigates results and restores trigger focus', async () => {
    const selected = vi.fn()
    const query = vi.fn()
    render(() => <CommandSearch defaultQuery="pro" results={[{ key: 'disabled', title: 'Disabled', disabled: true }, { key: 'product', title: 'Product', subtitle: 'Insurance' }, { key: 'policy', title: 'Policy' }]} onQueryChange={query} onSelect={selected} />)
    await fireEvent.keyDown(document, { key: 'k', metaKey: true })
    const input = screen.getByRole('searchbox')
    await Promise.resolve()
    expect(input).toHaveFocus()
    expect(document.querySelector('[data-command-key="product"]')).toHaveClass('bg-primary-500/8')
    await fireEvent.keyDown(input, { key: 'ArrowDown' })
    expect(document.querySelector('[data-command-key="policy"]')).toHaveClass('bg-primary-500/8')
    await fireEvent.keyDown(input, { key: 'Enter' })
    expect(selected).toHaveBeenCalledWith(expect.objectContaining({ key: 'policy' }), expect.any(KeyboardEvent))
    await fireEvent.input(input, { target: { value: 'policy' } })
    expect(query).toHaveBeenCalledWith('policy')
    await fireEvent.keyDown(input, { key: 'Escape' })
    await Promise.resolve()
    expect(screen.getByRole('button', { name: 'Search' })).toHaveFocus()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('renders command result, group and empty-state canonical surfaces', () => {
    render(() => <ul><CommandGroup>Products</CommandGroup><CommandResult result={{ key: 'one', title: 'Product', subtitle: 'CASCO', meta: 'Draft', badges: [<span>New</span>] }} highlighted /><CommandEmpty>No matches</CommandEmpty></ul>)
    expect(screen.getByText('Products')).toHaveClass('sticky', 'uppercase', 'tracking-[0.2em]', 'backdrop-blur')
    expect(screen.getByText('Product').closest('[data-command-item]')).toHaveClass('border-primary-300/30', 'bg-primary-500/8', 'shadow-sm')
    expect(screen.getByText('CASCO')).toHaveClass('text-xs', 'text-300')
    expect(screen.getByText('No matches').closest('li')).toHaveClass('px-5', 'py-8', 'text-center')
  })

  it('keeps command and kanban ids unique and lets one shortcut owner respond', async () => {
    render(() => <><CommandSearch defaultQuery="a" results={[{ key: 'a', title: 'First' }]} /><CommandSearch defaultQuery="b" results={[{ key: 'b', title: 'Second' }]} /><KanbanBoard><KanbanColumn columnKey="same" title="One" /><KanbanColumn columnKey="same" title="Two" /></KanbanBoard></>)
    await fireEvent.keyDown(document, { key: 'k', metaKey: true })
    expect(screen.getAllByRole('dialog')).toHaveLength(1)
    const ids = Array.from(document.querySelectorAll<HTMLElement>('[id]')).map((element) => element.id)
    expect(new Set(ids).size).toBe(ids.length)
  })
})
