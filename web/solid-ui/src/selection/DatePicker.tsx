import { createEffect, createMemo, createSignal, createUniqueId, For, onCleanup, onMount, Show, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'
import { callHandler } from '../internal/events'

export type DatePickerMode = 'single' | 'multiple' | 'range'
export type DateSelectorType = 'day' | 'month' | 'week' | 'year'

export interface DatePickerLabels {
  previous: string
  next: string
  calendar: string
}

export interface DatePickerProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'onChange'> {
  value?: readonly string[]
  defaultValue?: readonly string[]
  selected?: readonly string[]
  onValueChange?: (value: string[]) => void
  mode?: DatePickerMode
  selectorType?: DateSelectorType
  dateFormat?: string
  labelFormat?: Intl.DateTimeFormatOptions
  minDate?: string
  maxDate?: string
  locale?: string
  label?: JSX.Element
  placeholder?: string
  name?: string
  startName?: string
  endName?: string
  form?: string
  disabled?: boolean
  readOnly?: boolean
  required?: boolean
  inputProps?: Omit<JSX.InputHTMLAttributes<HTMLInputElement>, 'value' | 'name' | 'form'>
  labels?: Partial<DatePickerLabels>
}

const defaultLabels: DatePickerLabels = { previous: 'Previous', next: 'Next', calendar: 'Choose date' }

function localDate(year: number, month: number, day: number): Date {
  return new Date(year, month, day, 12)
}

function dateKey(date: Date): number {
  return Date.UTC(date.getFullYear(), date.getMonth(), date.getDate())
}

function addDays(date: Date, amount: number): Date {
  return localDate(date.getFullYear(), date.getMonth(), date.getDate() + amount)
}

function startOfWeek(date: Date): Date {
  const offset = (date.getDay() + 6) % 7
  return addDays(date, -offset)
}

function parseDate(value: string, format: string): Date | undefined {
  const numbers = value.match(/\d+/g)?.map(Number)
  if (!numbers || numbers.length < 3) return undefined
  const tokens = format.match(/[Ymd]/g)
  if (!tokens || tokens.length < 3) return undefined
  const parts: Partial<Record<'Y' | 'm' | 'd', number>> = {}
  tokens.forEach((token, index) => { parts[token as 'Y' | 'm' | 'd'] = numbers[index] })
  if (!parts.Y || !parts.m || !parts.d) return undefined
  const date = localDate(parts.Y, parts.m - 1, parts.d)
  return Number.isNaN(date.getTime()) ? undefined : date
}

function formatDate(date: Date, format: string): string {
  return format.replace(/Y|m|d/g, (token) => token === 'Y'
    ? String(date.getFullYear())
    : String(token === 'm' ? date.getMonth() + 1 : date.getDate()).padStart(2, '0'))
}

function normalizeForSelector(date: Date, selector: DateSelectorType): Date {
  if (selector === 'week') return startOfWeek(date)
  if (selector === 'month') return localDate(date.getFullYear(), date.getMonth(), 1)
  if (selector === 'year') return localDate(date.getFullYear(), 0, 1)
  return localDate(date.getFullYear(), date.getMonth(), date.getDate())
}

export function DatePicker(props: DatePickerProps) {
  const generatedID = createUniqueId()
  const [local, native] = splitProps(props, [
    'value', 'defaultValue', 'selected', 'onValueChange', 'mode', 'selectorType', 'dateFormat', 'labelFormat', 'minDate', 'maxDate',
    'locale', 'label', 'placeholder', 'name', 'startName', 'endName', 'form', 'disabled', 'readOnly', 'required', 'inputProps', 'labels', 'class', 'id', 'ref',
  ])
  const format = () => local.dateFormat ?? 'Y-m-d'
  const mode = () => local.mode ?? 'single'
  const selector = () => local.selectorType ?? 'day'
  const controlled = () => local.value
  const [internalValue, setInternalValue] = createSignal<string[]>([...(local.defaultValue ?? local.selected ?? [])])
  const [open, setOpen] = createSignal(false)
  const initialDate = parseDate((controlled() ?? internalValue())[0] ?? '', format()) ?? new Date()
  const [viewDate, setViewDate] = createSignal(localDate(initialDate.getFullYear(), initialDate.getMonth(), 1))
  const [activeDate, setActiveDate] = createSignal(initialDate)
  let root!: HTMLDivElement
  let input!: HTMLInputElement
  const id = () => local.id ?? generatedID
  const labels = () => ({ ...defaultLabels, ...local.labels })
  const values = createMemo(() => [...(controlled() ?? internalValue())])
  const selectedDates = createMemo(() => values().map((value) => parseDate(value, format())).filter((date): date is Date => Boolean(date)))
  const min = () => local.minDate ? parseDate(local.minDate, format()) : undefined
  const max = () => local.maxDate ? parseDate(local.maxDate, format()) : undefined
  const locale = () => local.locale ?? 'en'
  const inactive = () => Boolean(local.disabled || local.readOnly)
  const labelFormatter = () => new Intl.DateTimeFormat(locale(), local.labelFormat ?? { year: 'numeric', month: 'short', day: 'numeric' })
  const displayValue = createMemo(() => selectedDates().map((date) => {
    if (selector() === 'year') return String(date.getFullYear())
    if (selector() === 'month') return new Intl.DateTimeFormat(locale(), { year: 'numeric', month: 'long' }).format(date)
    if (selector() === 'week') {
      const end = addDays(date, 6)
      return `${labelFormatter().format(date)} — ${labelFormatter().format(end)}`
    }
    return labelFormatter().format(date)
  }).join(mode() === 'range' ? ' — ' : ', '))
  const dayNames = createMemo(() => Array.from({ length: 7 }, (_, index) => new Intl.DateTimeFormat(locale(), { weekday: 'short' }).format(addDays(localDate(2024, 0, 1), index))))
  const monthNames = createMemo(() => Array.from({ length: 12 }, (_, month) => new Intl.DateTimeFormat(locale(), { month: 'short' }).format(localDate(2024, month, 1))))
  const calendarDays = createMemo(() => {
    const first = localDate(viewDate().getFullYear(), viewDate().getMonth(), 1)
    return Array.from({ length: 42 }, (_, index) => addDays(startOfWeek(first), index))
  })
  const yearChoices = createMemo(() => Array.from({ length: 12 }, (_, index) => viewDate().getFullYear() - 5 + index))
  const periodEnd = (date: Date) => selector() === 'week'
    ? addDays(date, 6)
    : selector() === 'month'
      ? localDate(date.getFullYear(), date.getMonth() + 1, 0)
      : selector() === 'year'
        ? localDate(date.getFullYear(), 11, 31)
        : date
  const unavailable = (date: Date) => Boolean((min() && dateKey(periodEnd(date)) < dateKey(min()!)) || (max() && dateKey(date) > dateKey(max()!)))
  const selected = (date: Date) => selectedDates().some((item) => dateKey(item) === dateKey(date))
  const inRange = (date: Date) => {
    const range = selectedDates()
    return mode() === 'range' && range.length === 2 && dateKey(date) > dateKey(range[0]!) && dateKey(date) < dateKey(range[1]!)
  }
  const update = (next: string[]) => {
    if (controlled() === undefined) setInternalValue(next)
    local.onValueChange?.(next)
    root.dispatchEvent(new CustomEvent('date-selected', { bubbles: true, detail: next }))
  }
  const choose = (rawDate: Date) => {
    const date = normalizeForSelector(rawDate, selector())
    if (unavailable(date) || inactive()) return
    const nextValue = formatDate(date, format())
    if (mode() === 'multiple') {
      update(values().includes(nextValue) ? values().filter((value) => value !== nextValue) : [...values(), nextValue])
    } else if (mode() === 'range') {
      const existing = selectedDates()
      if (existing.length !== 1) update([nextValue])
      else {
        const ordered = [existing[0]!, date].sort((left, right) => dateKey(left) - dateKey(right)).map((item) => formatDate(item, format()))
        update(ordered)
        setOpen(false)
        if (input.isConnected) input.focus()
      }
    } else {
      update([nextValue])
      setOpen(false)
      if (input.isConnected) input.focus()
    }
    setActiveDate(date)
  }
  const shiftView = (amount: number) => {
    const current = viewDate()
    setViewDate(localDate(current.getFullYear() + (selector() === 'year' ? amount * 12 : 0), current.getMonth() + (selector() === 'year' ? 0 : amount), 1))
  }
  const moveActive = (key: string) => {
    const vertical = key === 'ArrowUp' || key === 'ArrowDown'
    const direction = key === 'ArrowLeft' || key === 'ArrowUp' ? -1 : 1
    let next: Date
    if (selector() === 'month') next = localDate(activeDate().getFullYear(), activeDate().getMonth() + direction * (vertical ? 3 : 1), 1)
    else if (selector() === 'year') next = localDate(activeDate().getFullYear() + direction * (vertical ? 3 : 1), 0, 1)
    else next = addDays(activeDate(), direction * (selector() === 'week' ? (vertical ? 28 : 7) : (vertical ? 7 : 1)))
    setActiveDate(next)
    setViewDate(localDate(next.getFullYear(), next.getMonth(), 1))
  }
  const keydown: JSX.EventHandler<HTMLInputElement, KeyboardEvent> = (event) => {
    if (event.key === 'ArrowDown' || event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      setOpen(true)
    } else if (event.key === 'Escape') {
      event.preventDefault()
      setOpen(false)
    }
    callHandler(local.inputProps?.onKeyDown, event)
  }
  const calendarKeydown: JSX.EventHandler<HTMLElement, KeyboardEvent> = (event) => {
    if (['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key)) {
      event.preventDefault()
      moveActive(event.key)
    } else if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      choose(activeDate())
    } else if (event.key === 'Escape') {
      event.preventDefault()
      setOpen(false)
      if (input.isConnected) input.focus()
    }
  }

  createEffect(() => {
    if (!open()) return
    activeDate()
    queueMicrotask(() => {
      if (!root.isConnected || !open()) return
      root.querySelector<HTMLElement>('[data-calendar-active="true"]')?.focus()
    })
  })
  onMount(() => {
    const outside = (event: PointerEvent) => {
      if (!root.contains(event.target as Node)) setOpen(false)
    }
    document.addEventListener('pointerdown', outside)
    onCleanup(() => document.removeEventListener('pointerdown', outside))
  })

  const cellClasses = (date: Date) => classes(
    selector() === 'month' ? 'flatpickr-monthSelect-month' : selector() === 'year' ? 'flatpickr-yearSelect-year' : 'flatpickr-day',
    selector() === 'day' && date.getMonth() !== viewDate().getMonth() && (dateKey(date) < dateKey(viewDate()) ? 'prevMonthDay' : 'nextMonthDay'),
    selected(date) && 'selected',
    mode() === 'range' && selectedDates().length > 0 && dateKey(date) === dateKey(selectedDates()[0]!) && 'startRange',
    mode() === 'range' && selectedDates().length === 2 && dateKey(date) === dateKey(selectedDates()[1]!) && 'endRange',
    inRange(date) && 'inRange',
    dateKey(date) === dateKey(new Date()) && 'today',
    unavailable(date) && 'flatpickr-disabled',
  )

  return (
    <div {...native} ref={(element) => { root = element; if (typeof local.ref === 'function') local.ref(element) }} id={id()} class={classes('relative', local.class)}>
      <Show when={local.label}><label class="form-control-label" for={`${id()}-input`}>{local.label}</label></Show>
      <div class="flex items-center w-full relative form-control">
        <input
          {...local.inputProps}
          ref={(element) => { input = element; if (typeof local.inputProps?.ref === 'function') local.inputProps.ref(element) }}
          id={`${id()}-input`}
          type="text"
          role="combobox"
          class={classes('form-control-input input outline-none w-full', local.inputProps?.class)}
          placeholder={local.placeholder}
          value={displayValue()}
          disabled={local.disabled}
          readOnly
          required={local.required}
          aria-haspopup="grid"
          aria-expanded={open()}
          aria-controls={`${id()}-calendar`}
          onClick={(event) => { if (!inactive()) setOpen(true); callHandler(local.inputProps?.onClick, event) }}
          onKeyDown={keydown}
        />
      </div>
      <Show when={mode() === 'range' && values().length === 2}>
        <div class="contents">
          <input type="hidden" data-datepicker-value name={local.startName || local.name} form={local.form} value={values()[0]} />
          <input type="hidden" data-datepicker-value name={local.endName || local.name} form={local.form} value={values()[1]} />
        </div>
      </Show>
      <Show when={mode() !== 'range'}><For each={values()}>{(value) => <input type="hidden" data-datepicker-value name={local.name} form={local.form} value={value} />}</For></Show>
      <Show when={open() && !inactive()}>
        <div id={`${id()}-calendar`} class={classes('flatpickr-calendar animate arrowTop open static', mode() === 'range' && 'rangeMode')} role="dialog" aria-label={labels().calendar}>
          <div class="flatpickr-months">
            <button type="button" class="flatpickr-prev-month" aria-label={labels().previous} onClick={() => shiftView(-1)}>‹</button>
            <div class="flatpickr-month"><div class="flatpickr-current-month"><span class="cur-month">{selector() === 'year' ? `${yearChoices()[0]}–${yearChoices().at(-1)}` : new Intl.DateTimeFormat(locale(), { month: 'long', year: 'numeric' }).format(viewDate())}</span></div></div>
            <button type="button" class="flatpickr-next-month" aria-label={labels().next} onClick={() => shiftView(1)}>›</button>
          </div>
          <Show when={selector() === 'day' || selector() === 'week'}>
            <div class="flatpickr-innerContainer">
              <div class="flatpickr-rContainer">
                <div class="flatpickr-weekdays"><div class="flatpickr-weekdaycontainer"><For each={dayNames()}>{(day) => <span class="flatpickr-weekday">{day}</span>}</For></div></div>
                <div class="flatpickr-days" role="grid" onKeyDown={calendarKeydown}>
                  <div class="dayContainer">
                    <For each={calendarDays()}>{(date) => <button type="button" role="gridcell" aria-selected={selected(normalizeForSelector(date, selector()))} aria-disabled={unavailable(date)} tabindex={dateKey(activeDate()) === dateKey(date) ? 0 : -1} data-calendar-active={dateKey(activeDate()) === dateKey(date)} class={classes(cellClasses(normalizeForSelector(date, selector())), selector() === 'week' && 'week')} disabled={unavailable(date)} onClick={() => choose(date)}>{date.getDate()}</button>}</For>
                  </div>
                </div>
              </div>
            </div>
          </Show>
          <Show when={selector() === 'month'}><div class="flatpickr-monthSelect-months" role="grid" onKeyDown={calendarKeydown}><For each={monthNames()}>{(month, index) => {
            const date = () => localDate(viewDate().getFullYear(), index(), 1)
            return <button type="button" role="gridcell" class={cellClasses(date())} aria-selected={selected(date())} disabled={unavailable(date())} tabindex={dateKey(activeDate()) === dateKey(date()) ? 0 : -1} data-calendar-active={dateKey(activeDate()) === dateKey(date())} onClick={() => choose(date())}>{month}</button>
          }}</For></div></Show>
          <Show when={selector() === 'year'}><div class="flatpickr-yearSelect-years" role="grid" onKeyDown={calendarKeydown}><For each={yearChoices()}>{(year) => {
            const date = () => localDate(year, 0, 1)
            return <button type="button" role="gridcell" class={cellClasses(date())} aria-selected={selected(date())} disabled={unavailable(date())} tabindex={dateKey(activeDate()) === dateKey(date()) ? 0 : -1} data-calendar-active={dateKey(activeDate()) === dateKey(date())} onClick={() => choose(date())}>{year}</button>
          }}</For></div></Show>
        </div>
      </Show>
    </div>
  )
}

export type DateRangePickerProps = Omit<DatePickerProps, 'mode'>

export function DateRangePicker(props: DateRangePickerProps) {
  return <DatePicker {...props} mode="range" />
}
