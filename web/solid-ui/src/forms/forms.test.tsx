import { afterEach, describe, expect, it, vi } from 'vitest'
import { createSignal } from 'solid-js'
import { render } from 'solid-js/web'
import { Button } from './Button'
import { Checkbox } from './Checkbox'
import { Input, MoneyInput, PasswordInput } from './Input'
import { Select } from './Select'
import { Textarea } from './Textarea'

let dispose: (() => void) | undefined
let host: HTMLDivElement

function mount(view: () => unknown) {
  host = document.createElement('div')
  document.body.append(host)
  dispose = render(view as never, host)
  return host
}

afterEach(() => {
  dispose?.()
  dispose = undefined
  document.body.replaceChildren()
})

describe('Button', () => {
  it('matches canonical classes and preserves native button attributes and refs', () => {
    let ref!: HTMLButtonElement
    mount(() => <Button variant="danger" size="sm" rounded loading disabled data-kind="save" ref={ref}>Save</Button>)
    const button = host.querySelector('button')!
    expect(button).toBe(ref)
    expect(button.className).toBe('shrink-0 btn cursor-pointer btn-danger btn-sm btn-rounded btn-loading btn-disabled')
    expect(button.disabled).toBe(true)
    expect(button.dataset.kind).toBe('save')
    expect(button.getAttribute('aria-busy')).toBe('true')
    expect(button.querySelector('.btn-loading-indicator')).not.toBeNull()
  })

  it('renders the canonical anchor form when href is supplied', () => {
    mount(() => <Button href="/products" variant="secondary" target="_blank">Products</Button>)
    const link = host.querySelector('a')!
    expect(link.getAttribute('href')).toBe('/products')
    expect(link.target).toBe('_blank')
    expect(link.classList.contains('btn-secondary')).toBe(true)
  })
})

describe('Input', () => {
  it.each(['text', 'number', 'email', 'tel', 'date', 'datetime-local'] as const)('renders %s with canonical field DOM', (type) => {
    mount(() => <Input type={type} id={`field-${type}`} label="Value" error="Required" name="value" data-native="yes" />)
    const input = host.querySelector('input')!
    expect(input.type).toBe(type)
    expect(input.className).toBe('form-control-input outline-none w-full')
    expect(input.dataset.native).toBe('yes')
    expect(host.querySelector('label')!.htmlFor).toBe(input.id)
    expect(input.getAttribute('aria-invalid')).toBe('true')
    expect(host.querySelector('[role="alert"]')!.id).toBe(`${input.id}-error`)
  })

  it('supports controlled and uncontrolled native values', () => {
    const changes = vi.fn()
    mount(() => {
      const [value, setValue] = createSignal('one')
      return <Input value={value()} onInput={(event) => { setValue(event.currentTarget.value); changes(event.currentTarget.value) }} />
    })
    const input = host.querySelector('input')!
    input.value = 'two'
    input.dispatchEvent(new InputEvent('input', { bubbles: true }))
    expect(changes).toHaveBeenCalledWith('two')
    expect(input.value).toBe('two')

    dispose?.()
    host.replaceChildren()
    dispose = render(() => <Input defaultValue="initial" />, host)
    expect(host.querySelector('input')!.value).toBe('initial')
  })

  it('centers select caret artwork inside its viewBox', () => {
    mount(() => <Select options={[{ value: 'one', label: 'One' }]} />)
    const caret = host.querySelector('select + svg')!
    expect(caret.getAttribute('viewBox')).toBe('0 0 16 16')
    expect(caret.querySelector('polyline')?.getAttribute('points')).toBe('3 6 8 10 13 6')
    expect(caret).toHaveClass('top-1/2')
    expect(caret).toHaveStyle({ transform: 'translateY(-50%)' })
  })

  it('keeps addons inside the canonical form-control wrapper', () => {
    mount(() => <Input addonLeft={<span>UZS</span>} addonRight={<span>%</span>} />)
    const wrapper = host.querySelector('.form-control')!
    expect(wrapper.firstElementChild!.className).toBe('flex pl-2.5')
    expect(wrapper.lastElementChild!.className).toBe('flex pr-2.5')
  })

  it('opens an optional field description next to the label', () => {
    mount(() => <Input label="Product code" description="Used by integrations" helpLabel="About product code" />)
    const help = host.querySelector<HTMLButtonElement>('[data-field-help] button')!
    expect(help.getAttribute('aria-label')).toBe('About product code')
    expect(host.querySelector('[role="dialog"]')).toBeNull()
    help.click()
    expect(host.querySelector('[role="dialog"]')!.textContent).toContain('Used by integrations')
  })

  it('toggles password visibility without losing native value', () => {
    mount(() => <PasswordInput defaultValue="secret" />)
    const password = host.querySelector('input:not(.password-lock)') as HTMLInputElement
    const toggle = host.querySelector('.password-lock') as HTMLInputElement
    expect(password.type).toBe('password')
    toggle.click()
    expect((host.querySelector('input:not(.password-lock)') as HTMLInputElement).type).toBe('text')
    expect((host.querySelector('input:not(.password-lock)') as HTMLInputElement).value).toBe('secret')
  })
})

describe('MoneyInput', () => {
  it('formats cents, submits the integer value, validates and reports changes', () => {
    const changed = vi.fn()
    mount(() => <MoneyInput name="amount" currency="UZS" value={123456} precision={2} min={100} max={200000} onValueChange={changed} />)
    const hidden = host.querySelector('input[type="hidden"]') as HTMLInputElement
    const visible = host.querySelector('input[type="text"]') as HTMLInputElement
    expect(hidden.name).toBe('amount')
    expect(hidden.value).toBe('123456')
    expect(visible.value).toBe('1,234.56')
    visible.value = '9,999.99'
    visible.dispatchEvent(new InputEvent('input', { bubbles: true }))
    expect(changed).toHaveBeenCalledWith(999999)
    expect(host.querySelector('[role="alert"]')!.textContent).toContain('Maximum')
  })
})

describe('Textarea, Select and Checkbox', () => {
  it('preserves textarea native attributes, value and error association', () => {
    mount(() => <Textarea id="notes" label="Notes" defaultValue="Hello" rows={4} error="Too short" />)
    const textarea = host.querySelector('textarea')!
    expect(textarea.value).toBe('Hello')
    expect(textarea.rows).toBe(4)
    expect(textarea.getAttribute('aria-describedby')).toBe('notes-error')
  })

  it('renders select prefix, placeholder, options and controlled value', () => {
    mount(() => <Select id="currency" prefix="Pay in" placeholder="Choose" value="UZS" options={[{ value: 'UZS', label: 'UZS' }, { value: 'USD', label: 'USD', disabled: true }]} />)
    const select = host.querySelector('select')!
    expect(select.value).toBe('UZS')
    expect(select.classList.contains('rounded-l-none')).toBe(true)
    expect(select.options).toHaveLength(3)
    expect(select.options[2]!.disabled).toBe(true)
  })

  it('applies a default value after options mount and keeps it uncontrolled', () => {
    mount(() => <Select defaultValue="voluntary" options={[{ value: 'mandatory', label: 'Mandatory' }, { value: 'voluntary', label: 'Voluntary' }]} />)
    const select = host.querySelector('select')!
    expect(select.value).toBe('voluntary')
    select.value = 'mandatory'
    select.dispatchEvent(new Event('change', { bubbles: true }))
    expect(select.value).toBe('mandatory')
  })

  it('reacts to controlled select value changes', () => {
    let setValue!: (value: string) => string
    mount(() => {
      const [value, updateValue] = createSignal('mandatory')
      setValue = updateValue
      return <Select value={value()} options={[{ value: 'mandatory', label: 'Mandatory' }, { value: 'voluntary', label: 'Voluntary' }]} />
    })
    const select = host.querySelector('select')!
    setValue('voluntary')
    expect(select.value).toBe('voluntary')
  })

  it('keeps a controlled value when asynchronous child options are replaced', () => {
    let replaceOptions!: () => void
    mount(() => {
      const [value, setValue] = createSignal('voluntary')
      const [options, setOptions] = createSignal([
        { value: 'mandatory', label: 'Mandatory' },
        { value: 'voluntary', label: 'Voluntary' },
      ])
      replaceOptions = () => setOptions([
        { value: 'mandatory', label: 'Mandatory refreshed' },
        { value: 'voluntary', label: 'Voluntary refreshed' },
      ])
      return (
        <Select value={value()} onChange={(event) => setValue(event.currentTarget.value)}>
          {options().map((option) => <option value={option.value}>{option.label}</option>)}
        </Select>
      )
    })
    const select = host.querySelector('select')!
    expect(select.value).toBe('voluntary')
    replaceOptions()
    expect(select.value).toBe('voluntary')
  })

  it('supports checked, indeterminate, native events and refs', () => {
    const changed = vi.fn()
    let ref!: HTMLInputElement
    mount(() => <Checkbox ref={ref} label="Active" indeterminate onChange={changed} />)
    const checkbox = host.querySelector('input')!
    expect(checkbox).toBe(ref)
    expect(checkbox.indeterminate).toBe(true)
    checkbox.click()
    expect(changed).toHaveBeenCalledOnce()
    expect(host.querySelector('label')!.htmlFor).toBe(checkbox.id)
  })

  it('keeps unchecked checkbox artwork empty', () => {
    mount(() => <Checkbox label="Inactive" />)
    const checkbox = host.querySelector('input')!
    const indicator = checkbox.nextElementSibling!
    expect(checkbox.checked).toBe(false)
    expect(indicator.classList.contains('iota-checkbox-indicator')).toBe(true)
    expect(indicator.querySelector('.iota-checkbox-check')).not.toBeNull()
    expect(indicator.querySelector('.iota-checkbox-minus')).not.toBeNull()
  })
})
