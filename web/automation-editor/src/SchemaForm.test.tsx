import { fireEvent, render, screen } from '@solidjs/testing-library'
import { createSignal } from 'solid-js'
import { describe, expect, it, vi } from 'vitest'
import type { CompletionPath } from './completion'
import { chatOfSelection, humanize, literalOfSelection, manualText, manualValue, parseDuration, SchemaForm, sourceModeOf, type SchemaFormProps } from './SchemaForm'
import { offeredParticipants } from './graph'
import type { AutomationGraph, CatalogEvent, JsonSchema } from './types'

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

describe('participants of a notify node', () => {
  const schema: JsonSchema = {
    type: 'object',
    properties: {
      recipients: {
        type: 'object',
        properties: {
          roles: { type: 'array', items: { type: 'string' } },
          participants: { type: 'array', items: { type: 'string' }, 'x-event-participants': true },
        },
      },
    },
  }
  const triggered = (event: string): AutomationGraph => ({
    name: 'g', run_as: 'owner', triggers: [{ id: 't', event }], nodes: [{ id: 'n', type: 't', params: {} }],
    edges: [{ from: { node: 't', outcome: 'fired' }, to: 'n' }],
  })
  const events = [
    { key: 'acme.policy.created.v1', participants: ['created_user', 'actor'] },
    { key: 'acme.policy.deleted.v1' },
  ] as CatalogEvent[]
  const shown = (container: HTMLElement) => container.querySelector('[data-param="recipients.participants"]') !== null

  it('offers the field when the trigger event has participants', () => {
    const { container } = mount(schema, {}, { graph: triggered('acme.policy.created.v1'), events })
    expect(shown(container)).toBe(true)
    expect(container.querySelector('[data-param="recipients.roles"]')).not.toBeNull()
  })

  it('hides it for a trigger event without a notification definition', () => {
    const { container } = mount(schema, {}, { graph: triggered('acme.policy.deleted.v1'), events })
    expect(shown(container)).toBe(false)
    expect(container.querySelector('[data-param="recipients.roles"]')).not.toBeNull()
  })

  it('hides it for a node no trigger reaches', () => {
    const lonely: AutomationGraph = { ...triggered('acme.policy.created.v1'), edges: [] }
    expect(shown(mount(schema, {}, { graph: lonely, events }).container)).toBe(false)
  })

  it('offers only what every trigger that reaches the node offers', () => {
    const both: AutomationGraph = {
      ...triggered('acme.policy.created.v1'),
      triggers: [{ id: 't', event: 'acme.policy.created.v1' }, { id: 'u', event: 'acme.policy.deleted.v1' }],
      edges: [{ from: { node: 't', outcome: 'fired' }, to: 'n' }, { from: { node: 'u', outcome: 'fired' }, to: 'n' }],
    }
    expect(shown(mount(schema, {}, { graph: both, events }).container)).toBe(false)
    expect(offeredParticipants(both, { events }, 'n')).toEqual([])
    expect(offeredParticipants(triggered('acme.policy.created.v1'), { events }, 'n')).toEqual(['created_user', 'actor'])
  })

  it('keeps a field that already holds a value, so the author can clear it', () => {
    const { container } = mount(schema, { recipients: { participants: ['actor'] } }, { graph: triggered('acme.policy.deleted.v1'), events })
    expect(shown(container)).toBe(true)
  })

  it('shows everything when the host passes no events', () => {
    expect(shown(mount(schema, {}, { graph: triggered('acme.policy.deleted.v1') }).container)).toBe(true)
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

  it('follows the stored value when it changes from outside, and keeps the mode while the field types', () => {
    const [value, setValue] = createSignal<Record<string, unknown>>({ chat_id: 700 })
    const { container } = render(() => (
      <SchemaForm
        schema={schema} value={value()} onChange={setValue} readOnly={false} locale="en" nodeType="t" nodeId="n" graph={graph}
        expressions={[]} templates={[]} completions={() => []} searchOptions={async () => []}
      />
    ))
    fireEvent.click(screen.getByRole('radio', { name: 'Expression' }))
    fireEvent.input(container.querySelector('[data-param="chat_id"] textarea') as HTMLTextAreaElement, { target: { value: 'event.chatId' } })
    expect(value().chat_id).toBe('event.chatId')
    // Undo puts the id back: the stored value is an id, not an expression, so the field reads as a list again.
    setValue({ chat_id: 700 })
    expect(screen.getByRole('radio', { name: 'From chats' })).toHaveAttribute('aria-checked', 'true')
    // While an id is typed by hand the mode stays manual.
    fireEvent.click(screen.getByRole('radio', { name: 'Id or @username' }))
    const input = () => container.querySelector('[data-param="chat_id"] input') as HTMLInputElement
    fireEvent.input(input(), { target: { value: '12' } })
    fireEvent.input(input(), { target: { value: '123' } })
    expect(input().value).toBe('123')
    expect(screen.getByRole('radio', { name: 'Id or @username' })).toHaveAttribute('aria-checked', 'true')
  })

  it('keeps the manual mode while a negative id or a username is typed one character at a time', () => {
    const [value, setValue] = createSignal<Record<string, unknown>>({})
    const { container } = render(() => (
      <SchemaForm
        schema={schema} value={value()} onChange={setValue} readOnly={false} locale="en" nodeType="t" nodeId="n" graph={graph}
        expressions={[]} templates={[]} completions={() => []} searchOptions={async () => []}
      />
    ))
    fireEvent.click(screen.getByRole('radio', { name: 'Id or @username' }))
    const input = () => container.querySelector('[data-param="chat_id"] input') as HTMLInputElement
    for (const text of ['-', '-1', '-10', '-1001']) fireEvent.input(input(), { target: { value: text } })
    expect(value().chat_id).toBe(-1001)
    expect(input().value).toBe('-1001')
    for (const text of ['@', '@s', '@sa', '@sales', '@salesteam']) {
      fireEvent.input(input(), { target: { value: text } })
      expect(screen.getByRole('radio', { name: 'Id or @username' })).toHaveAttribute('aria-checked', 'true')
      expect(input().value).toBe(text)
    }
    expect(value().chat_id).toBe('"@salesteam"')
  })

  it('reads a selection of the chat picker: an id, nothing when it was removed', () => {
    expect(chatOfSelection('-1001')).toBe(-1001)
    expect(chatOfSelection(['700'])).toBe(700)
    expect(chatOfSelection('')).toBeUndefined()
    expect(chatOfSelection(undefined)).toBeUndefined()
    expect(chatOfSelection('group:private')).toBeNull()
    expect(literalOfSelection('sales')).toBe('"sales"')
    expect(literalOfSelection('group:private')).toBeUndefined()
    expect(literalOfSelection('')).toBeUndefined()
  })

  it('asks again when the connection of the step changes', async () => {
    const searchOptions = vi.fn(async () => [{ value: '1', label: 'A', group: 'private' }])
    const [value, setValue] = createSignal<Record<string, unknown>>({ connection: 'one' })
    render(() => (
      <SchemaForm
        schema={{ type: 'object', properties: { chat_id: (schema.properties as Record<string, JsonSchema>).chat_id! } }} value={value()} onChange={setValue}
        readOnly={false} locale="en" nodeType="t" nodeId="n" graph={graph} expressions={[]} templates={[]} completions={() => []} searchOptions={searchOptions}
      />
    ))
    await vi.waitFor(() => expect(searchOptions).toHaveBeenCalledTimes(1))
    setValue({ connection: 'two' })
    await vi.waitFor(() => expect(searchOptions).toHaveBeenCalledTimes(2))
    expect(searchOptions).toHaveBeenLastCalledWith('telegram.chats', '', { nodeType: 't', params: { connection: 'two' } })
  })
})
