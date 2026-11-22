import { createSignal } from 'solid-js'
import { fireEvent, render, screen, within } from '@solidjs/testing-library'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { Action, ActionMenu, RowActions } from './Actions'
import { FilterBuilder, FilterChip, GroupedOptions, type FilterCondition } from './FilterBuilder'
import { FilterDropdown, FiltersBar, FiltersDrawer, SideFilter } from './Filters'
import { FormActions, FormLayout } from './Form'
import { DetailsDrawer, TableRowActions } from './TableDrawers'
import { DefaultFilters } from './DefaultFilters'

beforeEach(() => {
  HTMLDialogElement.prototype.showModal = function () { this.open = true }
  HTMLDialogElement.prototype.close = function () { this.open = false; this.dispatchEvent(new Event('close')) }
})

describe('scaffold presentation primitives', () => {
  it('renders canonical actions and action menus with typed callbacks', async () => {
    const create = vi.fn()
    const edit = vi.fn()
    render(() => <><RowActions><Action type="create" label="Create" onSelect={create} /><Action href="/export" label="Export" /></RowActions><ActionMenu label="More actions" trigger="More" defaultOpen actions={[{ label: 'Edit', onSelect: edit }]} /></>)
    expect(screen.getByText('Create').closest('button')).toHaveClass('btn-primary', 'btn-normal')
    expect(screen.getByText('Export').closest('a')).toHaveAttribute('href', '/export')
    expect(screen.getByText('Create').closest('.flex')).toHaveClass('gap-2')
    await fireEvent.click(screen.getByRole('menuitem', { name: 'Edit' }))
    expect(edit).toHaveBeenCalledOnce()
    await fireEvent.keyDown(screen.getByText('More').closest('summary')!, { key: 'Escape' })
    expect(screen.getByText('More').closest('details')).not.toHaveAttribute('open')
  })

  it('supports multi-select filter state, clear, Escape and exact menu classes', async () => {
    const changed = vi.fn()
    render(() => <FilterDropdown label="Status" name="status" defaultOpen defaultValue={['active']} onValueChange={changed} options={[{ value: 'active', label: 'Active' }, { value: 'closed', label: 'Closed' }]} />)
    const menu = screen.getByRole('list', { name: 'Status options' })
    expect(menu).toHaveClass('absolute', 'z-20', 'max-h-80', 'bg-white', 'border-subtle')
    const active = screen.getByLabelText('Active')
    active.focus()
    await fireEvent.keyDown(active, { key: 'ArrowDown' })
    expect(screen.getByLabelText('Closed')).toHaveFocus()
    await fireEvent.click(screen.getByLabelText('Closed'))
    expect(changed).toHaveBeenLastCalledWith(['active', 'closed'])
    await fireEvent.click(screen.getByRole('button', { name: 'Clear filter' }))
    expect(changed).toHaveBeenLastCalledWith([])
    await fireEvent.keyDown(menu.parentElement!, { key: 'Escape' })
    expect(screen.queryByRole('list', { name: 'Status options' })).not.toBeInTheDocument()
  })

  it('selects all side-filter values and preserves responsive toolbar layout', async () => {
    const changed = vi.fn()
    render(() => <><SideFilter name="kind" selectAllLabel="All kinds" options={[{ value: 'a', label: 'A' }, { value: 'b', label: 'B' }]} onValueChange={changed} /><FiltersBar search={<input aria-label="Search" />} actions={<button>New</button>}><span>Filters</span></FiltersBar></>)
    await fireEvent.click(screen.getByLabelText('All kinds'))
    expect(changed).toHaveBeenCalledWith(['a', 'b'])
    expect(screen.getByText('Filters').parentElement).toHaveClass('p-4', 'flex-col', 'md:flex-row', 'items-center')
    expect(screen.getByRole('button', { name: 'New' }).parentElement).toHaveClass('hidden', 'md:flex', 'ml-auto')
  })

  it('keeps scaffold form/card/footer DOM and exposes submit/delete callbacks', async () => {
    const submit = vi.fn((event: Event) => event.preventDefault())
    const remove = vi.fn()
    render(() => <FormLayout fields={<label>Code<input name="code" /></label>} onSubmit={submit} actions={<FormActions showDelete deleteLabel="Remove" saveLabel="Store" onDelete={remove} />} />)
    expect(screen.getByLabelText('Code').closest('.grid')).toHaveClass('grid-cols-1', 'md:grid-cols-2', 'gap-4')
    expect(screen.getByRole('button', { name: 'Store' }).parentElement).toHaveClass('h-16', 'md:h-20', 'shadow-t-lg', 'border-t-primary')
    await fireEvent.click(screen.getByRole('button', { name: 'Remove' }))
    expect(remove).toHaveBeenCalledOnce()
    await fireEvent.submit(screen.getByLabelText('Code').closest('form')!)
    expect(submit).toHaveBeenCalledOnce()
  })

  it('adds, edits and clears filter-builder conditions through typed state', async () => {
    const changes: FilterCondition[][] = []
    render(() => {
      const [conditions, setConditions] = createSignal<FilterCondition[]>([])
      return <FilterBuilder fields={[{ key: 'active', label: 'Active', type: 'bool' }, { key: 'amount', label: 'Amount', type: 'number', operators: ['is', 'between'] }]} conditions={conditions()} showResetAlways codec={{ encode: (condition) => `${condition.field}:${condition.values.join(',')}` }} onConditionsChange={(next) => { changes.push(next); setConditions(next) }} />
    })
    await fireEvent.click(screen.getByRole('button', { name: 'Add filter' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Active' }))
    expect(changes.at(-1)).toEqual([{ field: 'active', operator: 'is', values: ['true'] }])
    expect(document.querySelector('input[name="f"]')).toHaveValue('active:true')
    await fireEvent.click(screen.getByRole('button', { name: 'Clear all' }))
    expect(changes.at(-1)).toEqual([])
    await fireEvent.click(screen.getByRole('button', { name: 'Add filter' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Amount' }))
    expect(screen.getByRole('dialog')).toHaveClass('w-80', 'bg-surface-300', 'drop-shadow-sm')
    await fireEvent.input(screen.getByRole('spinbutton'), { target: { value: '250' } })
    await fireEvent.click(screen.getByRole('button', { name: 'Apply' }))
    expect(changes.at(-1)).toEqual([{ field: 'amount', operator: 'is', values: ['250'] }])
  })

  it('applies the empty operator of a nullable field without values', async () => {
    const changes: FilterCondition[][] = []
    render(() => <FilterBuilder fields={[{ key: 'project', label: 'Project', type: 'reference', nullable: true, options: [{ value: 'p1', label: 'Alpha' }] }]} onConditionsChange={(next) => changes.push(next)} />)
    await fireEvent.click(screen.getByRole('button', { name: 'Add filter' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Project' }))
    await fireEvent.change(screen.getByDisplayValue('is'), { target: { value: 'empty' } })
    expect(screen.queryByRole('listbox')).toBeNull()
    await fireEvent.click(screen.getByRole('button', { name: 'Apply' }))
    expect(changes.at(-1)).toEqual([{ field: 'project', operator: 'empty', values: [] }])
  })

  it('applies a date preset, keeps a required chip through Clear all and edits a flag chip', async () => {
    const changes: FilterCondition[][] = []
    render(() => {
      const [conditions, setConditions] = createSignal<FilterCondition[]>([
        { field: 'period', operator: 'between', values: ['2026-07-01', '2026-09-30'] },
        { field: 'overdue', operator: 'is', values: ['false'] },
      ])
      return <FilterBuilder
        fields={[
          { key: 'period', label: 'Period', type: 'date', operators: ['between'], required: true, presetOptions: [{ id: 'thisQuarter', label: 'This quarter', values: ['2026-10-01', '2026-12-31'] }] },
          { key: 'overdue', label: 'Overdue', type: 'bool' },
        ]}
        conditions={conditions()}
        operatorLabel={(op) => (op === 'between' ? '' : op)}
        boolLabels={{ yes: 'Yes', no: 'No' }}
        showResetAlways
        onConditionsChange={(next) => { changes.push(next); setConditions(next) }}
      />
    })
    expect(screen.getByRole('button', { name: /Period/ })).toHaveTextContent('2026-07-01 – 2026-09-30')
    expect(screen.getByRole('button', { name: /Overdue/ })).toHaveTextContent('No')
    expect(screen.getAllByRole('button', { name: 'Remove filter' })).toHaveLength(1)
    await fireEvent.click(screen.getByRole('button', { name: /Period/ }))
    await fireEvent.click(screen.getByRole('button', { name: 'This quarter' }))
    expect(changes.at(-1)![0]).toEqual({ field: 'period', operator: 'between', values: ['2026-10-01', '2026-12-31'], preset: 'thisQuarter' })
    expect(screen.getByRole('button', { name: /Period/ })).toHaveTextContent('This quarter')
    await fireEvent.click(screen.getByRole('button', { name: /Overdue/ }))
    await fireEvent.click(screen.getByRole('button', { name: 'Yes' }))
    expect(changes.at(-1)![1]).toEqual({ field: 'overdue', operator: 'is', values: ['true'] })
    await fireEvent.click(screen.getByRole('button', { name: 'Clear all' }))
    expect(changes.at(-1)!.map((condition) => condition.field)).toEqual(['period'])
  })

  it('keeps a flag field inside its operators and shows a negated flag as what it means', async () => {
    const changes: FilterCondition[][] = []
    render(() => {
      const [conditions, setConditions] = createSignal<FilterCondition[]>([
        { field: 'overdue', operator: 'is_not', values: ['false'] },
        { field: 'due', operator: 'before', values: ['2026-07-01'] },
      ])
      return <FilterBuilder
        fields={[
          { key: 'overdue', label: 'Overdue', type: 'bool', operators: ['is_not'] },
          { key: 'due', label: 'Due', type: 'date', operators: ['before'], presetOptions: [{ id: 'thisQuarter', label: 'This quarter', values: ['2026-10-01', '2026-12-31'] }] },
        ]}
        conditions={conditions()}
        addDialogLabel="Добавить фильтр"
        onConditionsChange={(next) => { changes.push(next); setConditions(next) }}
      />
    })
    // "is not false" reads as Yes, and Yes stays pressed in its editor.
    expect(screen.getByRole('button', { name: /Overdue/ })).toHaveTextContent('Yes')
    await fireEvent.click(screen.getByRole('button', { name: /Overdue/ }))
    expect(screen.getByRole('button', { name: 'Yes' })).toHaveAttribute('aria-pressed', 'true')
    await fireEvent.click(screen.getByRole('button', { name: 'No' }))
    expect(changes.at(-1)![0]).toEqual({ field: 'overdue', operator: 'is_not', values: ['true'] })
    expect(screen.getByRole('button', { name: /Overdue/ })).toHaveTextContent('No')
    // A date field without "between" offers no presets (they would apply "between").
    await fireEvent.click(screen.getByRole('button', { name: /Due/ }))
    expect(screen.queryByRole('button', { name: 'This quarter' })).not.toBeInTheDocument()
    await fireEvent.click(screen.getByRole('button', { name: /Add filter/ }))
    expect(screen.getByRole('dialog', { name: 'Добавить фильтр' })).toBeInTheDocument()
  })

  it('renders editable chips with canonical hidden codec inputs and removal', async () => {
    const edit = vi.fn()
    const remove = vi.fn()
    render(() => <FilterChip index={2} field={{ key: 'owner', label: 'Owner', type: 'reference' }} condition={{ field: 'owner', operator: 'is', values: ['Alice'] }} encoded="owner:is:Alice" summary="Alice" onEdit={edit} onRemove={remove} />)
    expect(document.querySelector('[data-fb-chip="2"]')).toHaveValue('owner:is:Alice')
    await fireEvent.click(screen.getByRole('button', { name: /Owner/ }))
    expect(edit).toHaveBeenCalledWith(2)
    await fireEvent.click(screen.getByRole('button', { name: 'Remove filter' }))
    expect(remove).toHaveBeenCalledWith(2)
  })

  it('renders details drawer values/actions and closes uncontrolled drawers', async () => {
    const closed = vi.fn()
    const action = vi.fn()
    render(() => <><DetailsDrawer title="Product" defaultOpen fields={[{ name: 'enabled', label: 'Enabled', type: 'boolean', value: true }, { name: 'state', label: 'State', type: 'badge', value: 'Draft' }]} actions={[{ label: 'Delete', method: 'delete', class: 'btn-danger', onSelect: action }]} onAfterClose={closed} /><FiltersDrawer heading="Filters" defaultOpen><span>Fields</span></FiltersDrawer><TableRowActions actions={[{ label: 'Open' }]} /></>)
    expect(screen.getByText('True')).toHaveClass('dark:bg-green-900/20', 'dark:text-green-400')
    expect(screen.getByText('Draft')).toHaveClass('bg-blue-50', 'dark:bg-blue-900/20')
    await fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    expect(action).toHaveBeenCalledOnce()
    const productDialog = screen.getByText('Product').closest('dialog')!
    expect(within(productDialog).getByText('Product')).toHaveClass('text-lg', 'font-medium')
    await fireEvent.click(within(productDialog).getByRole('button', { name: 'Close' }))
    expect(closed).toHaveBeenCalledOnce()
    expect(screen.getByText('Open').closest('.flex')).toHaveClass('gap-2')
  })

  it('keeps builder ids and grouped headings stable across reactive updates', async () => {
    render(() => {
      const [selected, setSelected] = createSignal<string[]>([])
      const fields = [{ key: 'active', label: 'Active', type: 'bool' as const }]
      return <><FilterBuilder fields={fields} /><FilterBuilder fields={fields} /><GroupedOptions aria-label="Owners" options={[{ value: 'a', label: 'Alice', group: 'People' }, { value: 'b', label: 'Bob', group: 'People' }]} selected={selected()} /><button onClick={() => setSelected(['b'])}>Select Bob</button></>
    })
    const builderIds = Array.from(document.querySelectorAll<HTMLElement>('[id^="filter-builder-"]')).map((element) => element.id)
    expect(new Set(builderIds).size).toBe(2)
    await fireEvent.click(screen.getByRole('button', { name: 'Select Bob' }))
    expect(screen.getByRole('option', { name: 'People' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Bob' })).toHaveProperty('selected', true)
  })

  it('preserves a consumer key handler while applying dropdown keyboard behavior', async () => {
    const keyed = vi.fn()
    render(() => <FilterDropdown label="Status" name="status" defaultOpen onKeyDown={keyed} options={[{ value: 'active', label: 'Active' }]} />)
    await fireEvent.keyDown(screen.getByLabelText('Active'), { key: 'Home' })
    expect(keyed).toHaveBeenCalledOnce()
  })

  it('generates collision-free form and action ids for repeated layouts', () => {
    render(() => <><FormLayout fields="First" actions={<FormActions showDelete />} /><FormLayout fields="Second" actions={<FormActions showDelete />} /></>)
    const ids = Array.from(document.querySelectorAll<HTMLElement>('[id]')).map((element) => element.id)
    expect(new Set(ids).size).toBe(ids.length)
  })

  it('maps all default filter presets into typed query state', async () => {
    const changed = vi.fn()
    render(() => <DefaultFilters fields={[{ key: 'name', label: 'Name' }, { key: 'code', label: 'Code' }]} now={new Date(2026, 8, 14)} onQueryChange={changed} />)
    await fireEvent.input(screen.getByPlaceholderText('Search'), { target: { value: 'CASCO' } })
    expect(changed).toHaveBeenLastCalledWith(expect.objectContaining({ search: 'CASCO' }))
    const selects = screen.getAllByRole('combobox')
    await fireEvent.change(selects.at(-1)!, { target: { value: 'this-week' } })
    expect(changed).toHaveBeenLastCalledWith(expect.objectContaining({ createdAtFrom: '2026-09-14', createdAtTo: '2026-09-20' }))
    expect(document.querySelector('input[name="CreatedAt.From"]')).toHaveValue('2026-09-14')
  })
})
