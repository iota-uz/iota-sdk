import { createSignal, type JSX } from 'solid-js'
import { fireEvent, render, screen } from '@solidjs/testing-library'
import { describe, expect, it, vi } from 'vitest'
import { BreadcrumbItem, BreadcrumbLink, BreadcrumbSeparator, Breadcrumbs } from './Breadcrumbs'
import { Dropdown, DropdownItem } from './Dropdown'
import { NavTabs, NavTabsContent, NavTabsList, NavTabsTrigger } from './NavTabs'
import { Pagination } from './Pagination'
import { BoostedContent, BoostedLink, Tabs, TabsContent, TabsList, TabsTrigger, type BoostedTabRequest } from './Tabs'

describe('navigation primitives', () => {
  it('renders canonical breadcrumb markup', () => {
    render(() => <Breadcrumbs aria-label="Breadcrumb"><BreadcrumbItem><BreadcrumbLink href="/products">Products</BreadcrumbLink></BreadcrumbItem><BreadcrumbSeparator /><BreadcrumbItem>New</BreadcrumbItem></Breadcrumbs>)
    expect(screen.getByRole('list')).toHaveClass('flex', 'items-center', 'gap-1', 'text-sm')
    expect(screen.getByRole('link')).toHaveClass('text-300')
    expect(screen.getByText('/')).toHaveAttribute('aria-hidden', 'true')
  })

  it('supports controlled tabs and wraparound keyboard selection', async () => {
    const changed = vi.fn()
    render(() => {
      const [value, setValue] = createSignal('details')
      return <Tabs value={value()} onValueChange={(next) => { setValue(next); changed(next) }}>
        <TabsList aria-label="Sections"><TabsTrigger value="details">Details</TabsTrigger><TabsTrigger value="pricing">Pricing</TabsTrigger></TabsList>
        <TabsContent value="details">Details panel</TabsContent><TabsContent value="pricing">Pricing panel</TabsContent>
      </Tabs>
    })
    const details = screen.getByRole('tab', { name: 'Details' })
    const pricing = screen.getByRole('tab', { name: 'Pricing' })
    details.focus()
    await fireEvent.keyDown(details, { key: 'ArrowLeft' })
    expect(pricing).toHaveFocus()
    expect(pricing).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tabpanel')).toHaveTextContent('Pricing panel')
    expect(changed).toHaveBeenCalledWith('pricing')
  })

  it('provides canonical animated navtabs with uncontrolled state', async () => {
    render(() => <NavTabs defaultValue="one"><NavTabsList><NavTabsTrigger value="one">One</NavTabsTrigger><NavTabsTrigger value="two">Two</NavTabsTrigger></NavTabsList><NavTabsContent value="one">First</NavTabsContent><NavTabsContent value="two">Second</NavTabsContent></NavTabs>)
    const two = screen.getByRole('tab', { name: 'Two' })
    await fireEvent.click(two)
    expect(two).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tabpanel')).toHaveTextContent('Second')
    expect(screen.getByRole('tablist')).toHaveClass('relative', 'bg-slate-800', 'rounded-2xl')
  })

  it('matches pagination window and disables boundary links', () => {
    render(() => <Pagination current={6} totalPages={20} href={(page) => `/products?page=${page}`} />)
    expect(screen.getByRole('link', { name: '6' })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('link', { name: 'Previous page' })).toHaveAttribute('href', '/products?page=5')
    expect(screen.getAllByText('...', { selector: 'a' })).toHaveLength(2)
    expect(screen.getAllByText('...', { selector: 'a' })[0]).toHaveClass('pointer-events-none')
  })

  it('supports controlled dropdown, outside dismissal and menu keyboard navigation', async () => {
    const changed = vi.fn()
    render(() => {
      const [open, setOpen] = createSignal(true)
      return <Dropdown trigger="Actions" open={open()} onOpenChange={(next) => { setOpen(next); changed(next) }}><DropdownItem>Edit</DropdownItem><DropdownItem>Delete</DropdownItem></Dropdown>
    })
    const trigger = screen.getByText('Actions')
    const edit = screen.getByRole('menuitem', { name: 'Edit' })
    edit.focus()
    await fireEvent.keyDown(edit, { key: 'ArrowDown' })
    expect(screen.getByRole('menuitem', { name: 'Delete' })).toHaveFocus()
    await fireEvent.keyDown(edit, { key: 'Escape' })
    expect(trigger).toHaveFocus()
    expect(changed).toHaveBeenCalledWith(false)
  })

  it('aborts stale boosted-tab loads and commits only the latest fragment', async () => {
    const requests: { request: BoostedTabRequest; resolve: (value: JSX.Element) => void }[] = []
    const loader = (request: BoostedTabRequest) => new Promise<JSX.Element>((resolve) => requests.push({ request, resolve }))
    render(() => <Tabs defaultValue="/one"><TabsList><BoostedLink href="/one" loader={loader}>One</BoostedLink><BoostedLink href="/two" loader={loader}>Two</BoostedLink></TabsList><BoostedContent fallback="Loading">Initial</BoostedContent></Tabs>)
    await fireEvent.click(screen.getByRole('button', { name: 'One' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Two' }))
    expect(requests[0]?.request.signal.aborted).toBe(true)
    requests[1]?.resolve(<strong>Second fragment</strong>)
    await Promise.resolve()
    requests[0]?.resolve(<strong>Stale fragment</strong>)
    await Promise.resolve()
    expect(screen.getByText('Second fragment')).toBeVisible()
    expect(screen.queryByText('Stale fragment')).not.toBeInTheDocument()
  })
})
