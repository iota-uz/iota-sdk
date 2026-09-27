import { createSignal } from 'solid-js'
import { fireEvent, render, screen } from '@solidjs/testing-library'
import { describe, expect, it, vi } from 'vitest'
import { Avatar, AvatarGroup, avatarColor } from './Avatar'
import { DescriptionList, DescriptionListDetails, DescriptionListItem, DescriptionListLabel, DescriptionListLink, DescriptionListValue } from './DescriptionList'
import { EmptyState, EmptyTableIllustration } from './EmptyState'
import { ContentLoader, Loader } from './Loader'
import { Body, Cell, Head, Header, Row, Table } from './Table'

describe('data and layout primitives', () => {
  it('renders the canonical responsive table DOM and exposes sort and row selection hooks', async () => {
    const sorted = vi.fn()
    const selected = vi.fn()
    render(() => (
      <Table aria-label="Products" wrapperClass="max-h-80" scrollbarGutter>
        <Header><Row><Head sortable sortDirection="asc" onSort={sorted}>Name</Head><Head priority={3}>Owner</Head></Row></Header>
        <Body><Row selected={false} onSelectedChange={selected}><Cell>Policy</Cell><Cell priority={3}>Alice</Cell></Row></Body>
      </Table>
    ))
    const table = screen.getByRole('table', { name: 'Products' })
    expect(table).toHaveClass('min-w-full', 'bg-surface-600', 'text-sm', 'rounded-lg')
    expect(table.parentElement).toHaveClass('overflow-x-auto', 'table-scrollbar-gutter', 'max-h-80')
    const sortable = screen.getByRole('columnheader', { name: 'Name' })
    expect(sortable).toHaveAttribute('aria-sort', 'ascending')
    await fireEvent.keyDown(sortable, { key: 'Enter' })
    expect(sorted).toHaveBeenCalledWith('desc', expect.any(KeyboardEvent))
    const row = screen.getByRole('row', { name: /Policy/ })
    await fireEvent.click(row)
    expect(selected).toHaveBeenCalledWith(true, expect.any(MouseEvent))
    expect(screen.getByRole('columnheader', { name: 'Owner' })).toHaveClass('max-lg:hidden')
    expect(screen.getByRole('cell', { name: 'Alice' })).toHaveClass('max-lg:hidden')
  })

  it('renders regular and nested description list contracts', () => {
    render(() => (
      <DescriptionList title="Product" subtitle="Details">
        <DescriptionListItem><DescriptionListLabel>Code</DescriptionListLabel><DescriptionListValue>CASCO</DescriptionListValue></DescriptionListItem>
        <DescriptionListItem><DescriptionListLabel>Terms</DescriptionListLabel><DescriptionListLink href="/terms">Open</DescriptionListLink></DescriptionListItem>
        <DescriptionListDetails label="More" open><span>Nested details</span></DescriptionListDetails>
      </DescriptionList>
    ))
    expect(screen.getByText('Product').parentElement).toHaveClass('flex', 'flex-col', 'gap-2', 'p-4')
    expect(screen.getByText('CASCO').closest('div')).toHaveClass('bg-gray-100', 'p-3')
    expect(screen.getByRole('link', { name: 'Open' })).toHaveClass('text-brand-500', 'underline')
    expect(screen.getByText('Nested details').closest('div')).toHaveClass('bg-surface-100', 'p-4')
  })

  it('renders the collapsible description list variant with canonical card classes', () => {
    render(() => <DescriptionList title="Customer" collapsible defaultOpen><DescriptionListItem>Data</DescriptionListItem></DescriptionList>)
    const details = screen.getByText('Customer').closest('details')
    expect(details).toHaveAttribute('open')
    expect(details).toHaveClass('bg-surface-300', 'rounded-lg', 'border-subtle', 'group')
  })

  it('keeps avatar sizing, shape, deterministic color, images and overflow accessible', () => {
    render(() => <><Avatar initials="DK" /><Avatar initials="AA" imageUrl="/avatar.png" imageAlt="Dilshod" variant="square" /><AvatarGroup aria-label="Reviewers" limit={2} avatars={[{ initials: 'AA' }, { initials: 'BB' }, { initials: 'CC' }]} /></>)
    const initials = screen.getByText('DK')
    expect(initials).toHaveClass('w-9', 'h-9', 'rounded-full')
    expect(initials).toHaveStyle({ backgroundColor: avatarColor('DK') })
    expect(screen.getByRole('img', { name: 'Dilshod' })).toHaveClass('w-9', 'h-9', 'rounded-lg', 'object-cover')
    expect(screen.getByRole('group', { name: 'Reviewers' })).toHaveClass('flex', '-space-x-2')
    expect(screen.getByLabelText('1 more')).toHaveTextContent('+1')
  })

  it('renders the exact empty and loading surfaces and swaps content reactively', async () => {
    const [loading, setLoading] = createSignal(true)
    render(() => <><EmptyState title="No products" description="Create the first one" /><ContentLoader loading={loading()}><button onClick={() => undefined}>Ready</button></ContentLoader><button onClick={() => setLoading(false)}>Resolve</button></>)
    expect(screen.getByText('No products').closest('div')?.parentElement).toHaveClass('py-36', 'px-4')
    expect(screen.getByText('No products').previousElementSibling).toHaveAttribute('width', '100')
    expect(screen.getByRole('status')).toContainElement(screen.getByText('Loading...'))
    await fireEvent.click(screen.getByRole('button', { name: 'Resolve' }))
    expect(screen.getByRole('button', { name: 'Ready' })).toBeVisible()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('allows a standalone loader label and size override', () => {
    render(() => <Loader label="Loading policies" spinnerClass="w-4 h-4" class="p-2" />)
    expect(screen.getByRole('status')).toHaveClass('p-2')
    expect(screen.getByText('Loading policies')).toHaveClass('sr-only')
    expect(screen.getByRole('status').querySelector('svg')).toHaveClass('w-4', 'h-4', 'animate-spin')
  })

  it('keeps SVG filter references local when illustrations repeat', () => {
    render(() => <><EmptyTableIllustration data-testid="first-empty" /><EmptyTableIllustration data-testid="second-empty" /></>)
    const filters = Array.from(document.querySelectorAll('filter'))
    expect(new Set(filters.map((filter) => filter.id)).size).toBe(filters.length)
    for (const svg of screen.getAllByTestId(/-empty$/)) {
      for (const group of svg.querySelectorAll('g[filter]')) {
        const id = group.getAttribute('filter')?.match(/^url\(#(.+)\)$/)?.[1]
        expect(Array.from(svg.querySelectorAll('filter')).some((filter) => filter.id === id)).toBe(true)
      }
    }
  })
})
