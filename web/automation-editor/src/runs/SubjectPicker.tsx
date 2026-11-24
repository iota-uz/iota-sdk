// The subject a dry run or a node test runs about: a search over the subjects of the graph's entity
// (EditorApi.searchSubjects). A graph whose triggers are about no entity says so.
import { Combobox, type ComboboxOption } from '@iota-uz/sdk/solid-ui'
import { createSignal, Show, type JSX } from 'solid-js'
import { catalogEvent } from '../graph'
import type { AutomationGraph, EditorApi, EditorCatalog, EditorLocale, Subject } from '../types'
import { runStrings } from './strings'

/** The subject entity of the graph's triggers ("none" when none is about one). */
export function subjectEntity(graph: AutomationGraph, catalog: EditorCatalog): string {
  for (const trigger of graph.triggers) {
    const entity = catalogEvent(catalog, trigger.event)?.subject
    if (entity && entity !== 'none') return entity
  }
  return 'none'
}

const keyOf = (s: Subject) => `${s.entity}:${s.key}`

export function SubjectPicker(props: {
  entity: string
  api: EditorApi
  locale: EditorLocale
  value: Subject | null
  onChange(subject: Subject | null): void
  disabled?: boolean
}): JSX.Element {
  const t = () => runStrings(props.locale).dry
  const labels = new Map<string, Subject>()
  const [options, setOptions] = createSignal<ComboboxOption[]>([])
  const load = async (query: string): Promise<ComboboxOption[]> => {
    if (!props.api.searchSubjects) return []
    const found = await props.api.searchSubjects(props.entity, query)
    const list = found.map((f) => {
      labels.set(keyOf(f.subject), { ...f.subject, label: f.label })
      return { value: keyOf(f.subject), label: f.note ? `${f.label} · ${f.note}` : f.label }
    })
    // A subject set from a run stays selectable.
    const current = props.value
    if (current && !list.some((o) => o.value === keyOf(current))) {
      labels.set(keyOf(current), current)
      list.unshift({ value: keyOf(current), label: current.label || current.key })
    }
    setOptions(list)
    return list
  }
  const shown = (): ComboboxOption[] => {
    const current = props.value
    if (!current || options().some((o) => o.value === keyOf(current))) return options()
    labels.set(keyOf(current), current)
    return [{ value: keyOf(current), label: current.label || current.key }, ...options()]
  }
  return (
    <Show when={props.entity !== 'none' && props.api.searchSubjects} fallback={<p class="ae-muted">{t().subject}: {t().subjectNone}</p>}>
      <Combobox
        label={t().subject}
        placeholder={t().subjectSearch}
        searchable
        disabled={props.disabled}
        options={shown()}
        loadOptions={(q) => load(q)}
        value={props.value ? keyOf(props.value) : ''}
        onValueChange={(v) => props.onChange(labels.get(typeof v === 'string' ? v : v[0] ?? '') ?? null)}
        data-testid="subject-picker"
      />
    </Show>
  )
}
