import { fireEvent, render, screen } from '@solidjs/testing-library'
import { createSignal } from 'solid-js'
import { describe, expect, it, vi } from 'vitest'
import type { CompletionPath } from './completion'
import { humanize, manualText, manualValue, parseDuration, SchemaForm, sourceModeOf, type SchemaFormProps } from './SchemaForm'
import type { AutomationGraph, JsonSchema } from './types'

const graph: AutomationGraph = { name: 'g', run_as: 'owner', triggers: [], nodes: [{ id: 'n', type: 't', params: {} }], edges: [] }

function mount(schema: JsonSchema, initial: Record<string, unknown>, extra: Partial<SchemaFormProps> = {}) {
  const onChange = vi.fn()
  const [value, setValue] = createSignal(initial)
  const result = render(() => (
    <SchemaForm
      schema={schema} value={value()} onChange={(next) => { onChange(next); setValue(next) }}
      readOnly={false} locale="en" nodeType="t" nodeId="n" graph={graph}
      expressions={[]} templates={[]} completions={() => []}
      {...extra}
    />
  ))
  return { ...result, onChange, value }
}

describe('SchemaForm', () => {
  it('labels fields humanly with the param name next to them', () => {
    mount({ type: 'object', properties: { missed_after: { type: 'string', maxLength: 20 } } }, {})
    expect(humanize('missed_after')).toBe('Missed after')
    expect(screen.getByText('Missed after')).toBeInTheDocument()
    expect(screen.getByText('missed_after').tagName).toBe('CODE')
  })

  it('renders a nullable string as a normal field and clears it to null', () => {
    const { onChange, container } = mount({ type: 'object', properties: { note: { type: ['string', 'null'], maxLength: 50 } } }, { note: 'hi' })
    const input = container.querySelector('[data-param="note"] input') as HTMLInputElement
    expect(input.type).toBe('text')
    fireEvent.input(input, { target: { value: '' } })
    expect(onChange).toHaveBeenLastCalledWith({ note: null })
  })

  it('shows nested fields of a nullable object with Clear', () => {
    const schema = { type: 'object', properties: { address: { type: ['object', 'null'], properties: { city: { type: 'string', maxLength: 40 } } } } }
    const { onChange, container } = mount(schema, { address: { city: 'Tashkent' } })
    expect((container.querySelector('[data-param="address.city"] input') as HTMLInputElement).value).toBe('Tashkent')
    fireEvent.click(screen.getByRole('button', { name: 'Clear' }))
    expect(onChange).toHaveBeenLastCalledWith({ address: null })
  })

  it('uses a combobox, not a native select, for more than five enum values', () => {
    const { container } = mount({ type: 'object', properties: { color: { enum: ['a', 'b', 'c', 'd', 'e', 'f'] } } }, {})
    const field = container.querySelector('[data-param="color"]')!
    expect(field.querySelector('[role="combobox"]')).not.toBeNull()
    expect(field.querySelector('select:not(.hidden)')).toBeNull()
  })

  it('uses radios for five or fewer enum values', () => {
    const { onChange } = mount({ type: 'object', properties: { mode: { enum: ['fast', 'slow'] } } }, {})
    fireEvent.click(screen.getByRole('radio', { name: 'slow' }))
    expect(onChange).toHaveBeenLastCalledWith({ mode: 'slow' })
  })

  it('shows JSON text only for a schema without a type, labelled any JSON', () => {
    const { container, unmount } = mount({ type: 'object', properties: { extra: {}, name: { type: 'string', maxLength: 10 } } }, {})
    expect(container.querySelectorAll('textarea').length).toBe(1)
    expect(screen.getByText('any JSON')).toBeInTheDocument()
    unmount()
    render(() => <SchemaForm schema={{ type: 'object', properties: { extra: {} } }} value={{}} onChange={() => {}} readOnly={false} locale="ru" nodeType="t" nodeId="n" graph={graph} expressions={[]} templates={[]} completions={() => []} />)
    expect(screen.getByText('любой JSON')).toBeInTheDocument()
  })

  it('lets the host replace a field through fieldSlot', () => {
    const slot = vi.fn((field: { param: string; onChange(v: unknown): void }) => field.param === 'connection'
      ? <button type="button" onClick={() => field.onChange('c1')}>Pick connection</button>
      : undefined)
    const { onChange } = mount({ type: 'object', properties: { connection: { type: 'string' }, subject: { type: 'string', maxLength: 10 } } }, {}, { fieldSlot: slot })
    fireEvent.click(screen.getByRole('button', { name: 'Pick connection' }))
    expect(onChange).toHaveBeenLastCalledWith({ connection: 'c1' })
    expect(screen.getByText('Subject')).toBeInTheDocument()
  })

  it('notes the host picker for connection without a slot', () => {
    mount({ type: 'object', properties: { connection: { type: 'string' } } }, {})
    expect(screen.getByText(/connection picker comes from the host/)).toBeInTheDocument()
  })

  it('edits a duration with an amount and a unit', () => {
    const { onChange, container } = mount({ type: 'object', properties: { timeout: { type: 'string', 'x-max-ms': 2592000000 } } }, { timeout: '2h' })
    const amount = container.querySelector('[data-param="timeout"] input') as HTMLInputElement
    expect(amount.value).toBe('2')
    fireEvent.input(amount, { target: { value: '5' } })
    expect(onChange).toHaveBeenLastCalledWith({ timeout: '5h' })
    fireEvent.change(container.querySelector('[data-param="timeout"] select')!, { target: { value: 'd' } })
    expect(onChange).toHaveBeenLastCalledWith({ timeout: '5d' })
    expect(parseDuration(90_000)).toEqual({ amount: '90', unit: 's' })
  })

  it('offers completions in an expression field and inserts the chosen path', () => {
    const paths: CompletionPath[] = [{ path: 'user.email', type: 'string', source: 'context' }, { path: 'user.age', type: 'int', source: 'context' }]
    const { container, onChange } = mount({ type: 'object', properties: { expression: { type: 'string' } } }, {}, {
      expressions: [{ param: 'expression', type: 'bool' }], completions: () => paths,
    })
    const area = container.querySelector('[data-param="expression"] textarea') as HTMLTextAreaElement
    area.value = 'user.e'
    area.setSelectionRange(6, 6)
    fireEvent.input(area)
    const options = screen.getAllByRole('option')
    expect(options.map((o) => o.textContent)).toEqual(['user.emailstring'])
    fireEvent.keyDown(area, { key: 'Enter' })
    expect(onChange).toHaveBeenLastCalledWith({ expression: 'user.email' })
    expect(screen.queryByRole('listbox')).toBeNull()
  })

  it('shows errors under their field', () => {
    mount({ type: 'object', properties: { name: { type: 'string', maxLength: 10 } } }, {}, { errors: [{ path: 'name', message: 'Required' }] })
    expect(screen.getByRole('alert')).toHaveTextContent('Required')
  })

  it('adds and removes list items', () => {
    const { onChange } = mount({ type: 'object', properties: { tags: { type: 'array', items: { type: 'string', maxLength: 20 } } } }, { tags: ['a'] })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    expect(onChange).toHaveBeenLastCalledWith({ tags: ['a', ''] })
    fireEvent.click(screen.getByRole('button', { name: 'Remove: Item 1' }))
    expect(onChange).toHaveBeenLastCalledWith({ tags: [''] })
  })
})

describe('a field with an options source (a chat)', () => {
  const schema: JsonSchema = {
    type: 'object',
    properties: { chat_id: { 'x-options-source': 'telegram.chats', oneOf: [{ type: 'integer' }, { type: 'string' }] } },
  }

  it('reads a stored value into its mode and writes the typed forms', () => {
    expect(sourceModeOf(700)).toBe('list')
    expect(sourceModeOf(undefined)).toBe('list')
    expect(sourceModeOf('"@salesteam"')).toBe('manual')
    expect(sourceModeOf('event.chatId')).toBe('expr')
    expect(manualValue(' -1001234567890 ')).toBe(-1001234567890)
    expect(manualValue('@salesteam')).toBe('"@salesteam"')
    expect(manualValue('')).toBeUndefined()
    expect(manualText('"@salesteam"')).toBe('@salesteam')
    expect(manualText(-100)).toBe('-100')
  })

  it('offers the known chats grouped, picks one as a number', async () => {
    const searchOptions = vi.fn(async () => [
      { value: '700', label: 'Alice', group: 'private' },
      { value: '-1001', label: 'Sales team (@salesteam)', group: 'group' },
    ])
    const { onChange, container } = mount(schema, {}, { searchOptions })
    const field = container.querySelector('[data-param="chat_id"]')!
    expect(screen.getByRole('radio', { name: 'From chats' })).toHaveAttribute('aria-checked', 'true')
    fireEvent.click(field.querySelector('[role="combobox"], input') as HTMLElement)
    expect(await screen.findByText('Private')).toBeInTheDocument()
    expect(screen.getByText('Groups')).toBeInTheDocument()
    fireEvent.click(await screen.findByRole('option', { name: 'Sales team (@salesteam)' }))
    expect(onChange).toHaveBeenLastCalledWith({ chat_id: -1001 })
    expect(searchOptions).toHaveBeenCalledWith('telegram.chats', '', { nodeType: 't', params: {} })
  })

  it('takes an id, a username or an expression by hand', () => {
    const { onChange, container } = mount(schema, {}, { searchOptions: async () => [] })
    fireEvent.click(screen.getByRole('radio', { name: 'Id or @username' }))
    const input = container.querySelector('[data-param="chat_id"] input') as HTMLInputElement
    fireEvent.input(input, { target: { value: '12345' } })
    expect(onChange).toHaveBeenLastCalledWith({ chat_id: 12345 })
    fireEvent.input(input, { target: { value: '@salesteam' } })
    expect(onChange).toHaveBeenLastCalledWith({ chat_id: '"@salesteam"' })
    fireEvent.click(screen.getByRole('radio', { name: 'Expression' }))
    expect(container.querySelector('[data-param="chat_id"] textarea')).not.toBeNull()
  })

  it('without a host list says so and still takes an id', () => {
    mount(schema, {})
    expect(screen.getByText(/list is not available here/)).toBeInTheDocument()
  })
})
