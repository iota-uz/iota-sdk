import { createMemo, createSignal, For, onCleanup, Show, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'

export type MultiLangValue = Record<string, string>
export const commonLocales = ['en', 'ru', 'uz', 'uz-cyrl'] as const

export interface MultiLangLabels {
  addLanguage: string
  removeLanguage: string
  textPlaceholder: string
  duplicateLocale: string
  invalidLocale: string
  confirmRemoval: string
  noTranslations: string
  empty: string
  more: (count: number) => string
}

const defaultLabels: MultiLangLabels = {
  addLanguage: '+ Add Language', removeLanguage: 'Remove this language', textPlaceholder: 'Enter text in this language',
  duplicateLocale: 'Duplicate language code', invalidLocale: 'Invalid format', confirmRemoval: 'Remove this translation?',
  noTranslations: 'No translations available', empty: '—', more: (count) => `+${count} more`,
}

export function isValidLocaleCode(locale: string): boolean {
  return /^[a-z]{2,5}(-[a-z]{2,8})?$/.test(locale)
}

export function localizedMultiLangValue(value: MultiLangValue, locale = 'en'): string {
  const normalized = locale.toLowerCase()
  if (value[normalized]) return value[normalized]
  const priorities = normalized === 'ru' ? ['uz', 'uz-cyrl', 'en'] : normalized === 'uz' ? ['uz-cyrl', 'ru', 'en'] : normalized === 'uz-cyrl' ? ['uz', 'ru', 'en'] : ['uz', 'uz-cyrl', 'ru']
  for (const candidate of priorities) if (value[candidate]) return value[candidate]!
  return Object.values(value).find(Boolean) ?? ''
}

interface LocaleRow { id: number; locale: string; value: string }
let nextLocaleRowID = 0

export interface MultiLangFormInputProps extends Omit<JSX.FieldsetHTMLAttributes<HTMLFieldSetElement>, 'value' | 'onChange'> {
  name: string
  label: JSX.Element
  value?: MultiLangValue
  defaultValue?: MultiLangValue
  onValueChange?: (value: MultiLangValue, valid: boolean) => void
  confirmRemoval?: (locale: string, value: string) => boolean | Promise<boolean>
  labels?: Partial<MultiLangLabels>
}

function rowsFromValue(value: MultiLangValue): LocaleRow[] {
  const rows: LocaleRow[] = commonLocales.map((locale) => ({ id: nextLocaleRowID++, locale, value: value[locale] ?? '' }))
  for (const [locale, translation] of Object.entries(value)) if (!commonLocales.includes(locale as never) && translation) rows.push({ id: nextLocaleRowID++, locale, value: translation })
  return rows
}

export function MultiLangFormInput(props: MultiLangFormInputProps) {
  const [local, native] = splitProps(props, ['name', 'label', 'value', 'defaultValue', 'onValueChange', 'confirmRemoval', 'labels', 'class'])
  const [rows, setRows] = createSignal(rowsFromValue(local.defaultValue ?? local.value ?? {}))
  let alive = true
  onCleanup(() => { alive = false })
  const labels = () => ({ ...defaultLabels, ...local.labels })
  const activeRows = createMemo(() => local.value === undefined ? rows() : rowsFromControlled(local.value, rows()))
  const errors = createMemo(() => {
    const counts = new Map<string, number>()
    for (const row of activeRows()) if (row.locale) counts.set(row.locale, (counts.get(row.locale) ?? 0) + 1)
    return new Map(activeRows().map((row) => [row.id, !row.locale || isValidLocaleCode(row.locale) ? (counts.get(row.locale) ?? 0) > 1 ? labels().duplicateLocale : '' : labels().invalidLocale]))
  })
  const serialized = createMemo(() => {
    const result: MultiLangValue = {}
    if ([...errors().values()].some(Boolean)) return { value: result, valid: false }
    for (const row of activeRows()) {
      const locale = row.locale.trim().toLowerCase().replace(/[\x00-\x1f\x7f]/g, '').slice(0, 20)
      const value = row.value.trim().replace(/[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]/g, '').slice(0, 1000)
      if (locale && value && isValidLocaleCode(locale)) result[locale] = value
    }
    return { value: result, valid: true }
  })
  const updateRows = (next: LocaleRow[]) => {
    if (local.value === undefined) setRows(next)
    const result: MultiLangValue = {}
    let valid = true
    const locales = next.map((row) => row.locale).filter(Boolean)
    next.forEach((row) => {
      if (row.locale && (!isValidLocaleCode(row.locale) || locales.filter((item) => item === row.locale).length > 1)) valid = false
      if (row.locale && row.value && isValidLocaleCode(row.locale)) result[row.locale] = row.value.trim().slice(0, 1000)
    })
    local.onValueChange?.(result, valid)
  }
  const update = (id: number, field: 'locale' | 'value', value: string) => updateRows(activeRows().map((row) => row.id === id ? { ...row, [field]: field === 'locale' ? value.toLowerCase().trim() : value } : row))
  const add = () => {
    const used = new Set(activeRows().map((row) => row.locale))
    const candidates = ['en', 'ru', 'uz', 'uz-cyrl', 'es', 'fr', 'de', 'it', 'pt', 'ja', 'ko', 'zh', 'ar']
    updateRows([...activeRows(), { id: nextLocaleRowID++, locale: candidates.find((locale) => !used.has(locale)) ?? '', value: '' }])
  }
  const remove = async (row: LocaleRow) => {
    if (row.value.trim()) {
      const accepted = local.confirmRemoval ? await local.confirmRemoval(row.locale, row.value) : window.confirm(labels().confirmRemoval)
      if (!alive || !accepted) return
    }
    updateRows(activeRows().filter((item) => item.id !== row.id))
  }
  return <fieldset {...native} class={classes('multilang-field border border-subtle rounded-lg p-4', local.class)}>
    <legend class="text-sm font-medium text-gray-900 px-2">{local.label}</legend>
    <div class="multilang-inputs space-y-3" data-field={local.name}>
      <For each={activeRows()}>{(row) => <div class="locale-row flex items-center space-x-2">
        <div class="relative"><input type="text" class={classes('locale-code w-16 px-2 py-1 text-xs border border-default rounded uppercase', errors().get(row.id) && 'border-red-500')} value={row.locale} placeholder={row.locale || 'en'} maxlength="14" aria-invalid={Boolean(errors().get(row.id))} onChange={(event) => update(row.id, 'locale', event.currentTarget.value)} onBlur={(event) => update(row.id, 'locale', event.currentTarget.value)} /><Show when={errors().get(row.id)}>{(error) => <div class="validation-error text-xs text-red-600 mt-1" role="alert">{error()}</div>}</Show></div>
        <input type="text" class="locale-value flex-1 px-3 py-2 border border-default rounded-md focus:ring-primary-500 focus:border-brand" value={row.value} placeholder={labels().textPlaceholder} onInput={(event) => update(row.id, 'value', event.currentTarget.value)} />
        <button type="button" class="remove-locale-btn text-red-600 hover:text-red-800 text-sm font-bold w-6 h-6" title={labels().removeLanguage} aria-label={`${labels().removeLanguage}: ${row.locale}`} onClick={() => void remove(row)}>×</button>
      </div>}</For>
    </div>
    <div class="mt-3"><button type="button" class="add-locale-btn text-sm text-blue-600 hover:text-blue-800" onClick={add}>{labels().addLanguage}</button></div>
    <input type="hidden" name={local.name} class="multilang-json" value={JSON.stringify(serialized().value)} />
  </fieldset>
}

function rowsFromControlled(value: MultiLangValue, existing: LocaleRow[]): LocaleRow[] {
  const locales = [...commonLocales, ...Object.keys(value).filter((locale) => !commonLocales.includes(locale as never) && value[locale])]
  return locales.map((locale) => ({ id: existing.find((row) => row.locale === locale)?.id ?? nextLocaleRowID++, locale, value: value[locale] ?? '' }))
}

export const FormInput = MultiLangFormInput

export interface MultiLangDetailsProps extends JSX.HTMLAttributes<HTMLDivElement> { value: MultiLangValue; labels?: Partial<MultiLangLabels> }
export function MultiLangDetailsView(props: MultiLangDetailsProps) {
  const entries = () => Object.entries(props.value).filter(([, value]) => value)
  const labels = () => ({ ...defaultLabels, ...props.labels })
  return <Show when={entries().length} fallback={<span class="text-gray-400">{labels().noTranslations}</span>}><div class={classes('multilang-details', props.class)}><For each={entries()}>{([locale, value]) => <div class="locale-item mb-2"><span class="inline-block w-8 text-xs font-medium text-gray-600 uppercase">{locale}:</span><span class="ml-2 text-gray-900">{value}</span></div>}</For></div></Show>
}
export const DetailsView = MultiLangDetailsView

export interface MultiLangDetailsCompactProps extends JSX.HTMLAttributes<HTMLDivElement> { value: MultiLangValue; locale?: string; labels?: Partial<MultiLangLabels> }
export function MultiLangDetailsCompact(props: MultiLangDetailsCompactProps) {
  const entries = () => Object.entries(props.value).filter(([, value]) => value)
  const primary = () => localizedMultiLangValue(props.value, props.locale)
  const labels = () => ({ ...defaultLabels, ...props.labels })
  const others = () => entries().filter(([, value]) => value !== primary())
  return <Show when={entries().length} fallback={<span class="text-gray-400">{labels().empty}</span>}><div class={classes('multilang-compact', props.class)}><div class="font-medium">{primary()}</div><Show when={others().length}><div class="text-xs text-gray-300 mt-1">{labels().more(others().length)}: {others().map(([locale, value]) => `${locale}: ${value}`).join(', ')}</div></Show></div></Show>
}
export const DetailsViewCompact = MultiLangDetailsCompact

export interface MultiLangTableCellProps extends JSX.HTMLAttributes<HTMLSpanElement> { value: MultiLangValue; locale?: string; emptyLabel?: string }
export function MultiLangTableCell(props: MultiLangTableCellProps) {
  const entries = () => Object.entries(props.value).filter(([, value]) => value)
  return <Show when={entries().length} fallback={<span class="text-gray-400">{props.emptyLabel ?? '—'}</span>}><span class={classes('multilang-cell', props.class)} title={entries().length > 1 ? entries().map(([locale, value]) => `${locale}: ${value}`).join(' | ') : entries()[0]?.[1]}>{localizedMultiLangValue(props.value, props.locale)}</span></Show>
}
export const TableCell = MultiLangTableCell
