import { afterEach, describe, expect, it, vi } from 'vitest'
import { createSignal } from 'solid-js'
import { render } from 'solid-js/web'
import { Combobox } from './Combobox'
import { CountriesSelect, countryCodes, getCountryOptions } from './Countries'
import { DatePicker, DateRangePicker } from './DatePicker'
import { SearchSelect } from './SearchSelect'

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
  vi.useRealTimers()
})

describe('Combobox', () => {
  it('keeps the canonical classes and owns a multiple selection', () => {
    const changed = vi.fn()
    mount(() => <Combobox multiple searchable name="team" label="Team" placeholder="Choose" defaultValue={['one']} options={[
      { value: 'one', label: 'One' }, { value: 'two', label: 'Two', count: 2 }, { value: 'off', label: 'Disabled', disabled: true },
    ]} onValueChange={changed} />)
    const input = host.querySelector<HTMLInputElement>('[role="combobox"]')!
    expect(input.closest('.form-control')).not.toBeNull()
    expect(host.querySelector('.combobox-dropdown')).toBeNull()
    input.click()
    const options = host.querySelectorAll<HTMLElement>('[role="option"]')
    expect(options[0]!.className).toContain('combobox-option')
    options[1]!.click()
    expect(changed).toHaveBeenLastCalledWith(['one', 'two'])
    expect(host.querySelectorAll('li.bg-surface-100')).toHaveLength(2)
    expect((host.querySelector('select') as HTMLSelectElement).multiple).toBe(true)
  })

  it('skips disabled options with the keyboard and closes on Escape and outside press', () => {
    const changed = vi.fn()
    mount(() => <Combobox searchable options={[
      { value: 'off', label: 'Disabled', disabled: true }, { value: 'on', label: 'Enabled' },
    ]} onValueChange={changed} />)
    const input = host.querySelector<HTMLInputElement>('[role="combobox"]')!
    input.focus()
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }))
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
    expect(changed).toHaveBeenCalledWith('on')
    input.click()
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(host.querySelector('[role="listbox"]')).toBeNull()
    input.click()
    document.body.dispatchEvent(new Event('pointerdown', { bubbles: true }))
    expect(host.querySelector('[role="listbox"]')).toBeNull()
  })

  it('debounces an async source and cancels it on replacement', async () => {
    vi.useFakeTimers()
    const load = vi.fn(async (query: string) => [{ value: query, label: query.toUpperCase() }])
    mount(() => <Combobox searchable loadOptions={load} />)
    const input = host.querySelector<HTMLInputElement>('[role="combobox"]')!
    input.value = 'al'
    input.dispatchEvent(new InputEvent('input', { bubbles: true }))
    await vi.advanceTimersByTimeAsync(250)
    expect(load).toHaveBeenCalledOnce()
    expect(host.querySelector('[role="option"]')?.textContent).toContain('AL')
  })
})

describe('SearchSelect', () => {
  it('loads after the minimum query, supports keyboard selection and submits the id', async () => {
    vi.useFakeTimers()
    const changed = vi.fn()
    const load = vi.fn(async () => [{ value: '42', label: 'Answer' }])
    mount(() => <SearchSelect name="answer" loadOptions={load} onValueChange={changed} debounceMs={20} />)
    const input = host.querySelector<HTMLInputElement>('[role="combobox"]')!
    input.value = 'a'
    input.dispatchEvent(new InputEvent('input', { bubbles: true }))
    await vi.advanceTimersByTimeAsync(20)
    expect(load).not.toHaveBeenCalled()
    input.value = 'an'
    input.dispatchEvent(new InputEvent('input', { bubbles: true }))
    await vi.advanceTimersByTimeAsync(20)
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
    expect(changed).toHaveBeenCalledWith('42', expect.objectContaining({ label: 'Answer' }))
    expect((host.querySelector('input[type="hidden"]') as HTMLInputElement).value).toBe('42')
    expect(input.value).toBe('Answer')
  })

  it('reports localized empty and error states', async () => {
    vi.useFakeTimers()
    mount(() => <SearchSelect loadOptions={async () => { throw new Error('offline') }} debounceMs={1} labels={{ loadError: 'Ошибка загрузки' }} />)
    const input = host.querySelector<HTMLInputElement>('[role="combobox"]')!
    input.value = 'query'
    input.dispatchEvent(new InputEvent('input', { bubbles: true }))
    await vi.advanceTimersByTimeAsync(1)
    expect(host.querySelector('[role="alert"]')).toHaveTextContent('Ошибка загрузки')
  })

  it('forwards refs and input events and follows a controlled selected option', () => {
    const inputEvent = vi.fn()
    let outer!: HTMLDivElement
    let input!: HTMLInputElement
    let select!: (option: { value: string; label: string }) => void
    mount(() => {
      const [option, setOption] = createSignal({ value: '1', label: 'One' })
      select = setOption
      return <SearchSelect ref={outer} value={option().value} selectedOption={option()} inputProps={{ ref: (element) => { input = element }, onInput: inputEvent }} />
    })
    expect(outer).toBe(host.firstElementChild)
    expect(input.value).toBe('One')
    input.value = 'query'
    input.dispatchEvent(new InputEvent('input', { bubbles: true }))
    expect(inputEvent).toHaveBeenCalledOnce()
    select({ value: '2', label: 'Two' })
    expect(input.value).toBe('Two')
  })
})

describe('DatePicker', () => {
  it('submits exactly two canonical hidden values for a completed range', () => {
    const changed = vi.fn()
    const selectedEvent = vi.fn()
    mount(() => <DateRangePicker defaultValue={['2026-09-10']} startName="from" endName="to" locale="en" onValueChange={changed} on:date-selected={selectedEvent} />)
    const input = host.querySelector<HTMLInputElement>('[role="combobox"]')!
    input.click()
    const day = [...host.querySelectorAll<HTMLButtonElement>('[role="gridcell"]')].find((cell) => cell.textContent === '15' && !cell.classList.contains('nextMonthDay') && !cell.classList.contains('prevMonthDay'))!
    day.click()
    expect(changed).toHaveBeenCalledWith(['2026-09-10', '2026-09-15'])
    const hidden = [...host.querySelectorAll<HTMLInputElement>('[data-datepicker-value]')]
    expect(hidden.map((item) => [item.name, item.value])).toEqual([['from', '2026-09-10'], ['to', '2026-09-15']])
    expect(selectedEvent).toHaveBeenCalledOnce()
  })

  it('uses flatpickr DOM classes, localizes labels and honors date bounds', () => {
    mount(() => <DatePicker defaultValue={['2026-09-10']} locale="ru" minDate="2026-09-05" maxDate="2026-09-20" />)
    const input = host.querySelector<HTMLInputElement>('[role="combobox"]')!
    expect(input.value).toMatch(/сент/i)
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }))
    expect(host.querySelector('.flatpickr-calendar.open')).not.toBeNull()
    const disabledDay = [...host.querySelectorAll<HTMLButtonElement>('.flatpickr-day')].find((cell) => cell.textContent === '1' && !cell.classList.contains('prevMonthDay'))!
    expect(disabledDay.disabled).toBe(true)
    expect(host.querySelectorAll('.flatpickr-weekday')).toHaveLength(7)
  })

  it('supports month and year selectors', () => {
    mount(() => <DatePicker selectorType="month" defaultValue={['2026-09-01']} />)
    host.querySelector<HTMLInputElement>('[role="combobox"]')!.click()
    expect(host.querySelectorAll('.flatpickr-monthSelect-month')).toHaveLength(12)
  })
})

describe('CountriesSelect', () => {
  it('matches the canonical country set and accepts server translation labels', () => {
    const changed = vi.fn()
    mount(() => <CountriesSelect name="country" defaultValue="UZ" locale="ru" countries={['UZ', 'US']} countryLabels={{ UZ: 'Узбекистан' }} onValueChange={changed} />)
    const select = host.querySelector('select')!
    expect(select.className).toContain('form-control-input')
    expect(select.options[0]!.textContent).toBe('Узбекистан')
    select.value = 'US'
    select.dispatchEvent(new Event('change', { bubbles: true }))
    expect(changed).toHaveBeenCalledWith('US')
    expect(countryCodes).toHaveLength(195)
    expect(getCountryOptions('en', ['IDN'])[0]).toEqual({ code: 'IDN', label: 'IDN' })
  })
})
