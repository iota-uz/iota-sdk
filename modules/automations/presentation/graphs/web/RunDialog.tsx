import { For, Match, Show, Switch, createMemo, createSignal, type JSX } from 'solid-js'
import { Button, Dialog, DialogBusyStatus, Input, Select, Switch as Toggle, Textarea } from '@iota-uz/sdk/solid-ui'
import { useApp } from './context'
import { Icon } from './Icon'
import { errorText, graphPath, inputFields, inputValue, type ManualTrigger } from './model'

/**
 * «Run» of a graph with a manual trigger (AL9): the input form by the trigger's schema with solid-ui
 * fields and, optionally, the subject of the run. A second click while the request runs does nothing
 * (F1); each start is a new run, announced by a toast that opens it (F5).
 */
export function RunDialog(props: { id: string; triggers: ManualTrigger[]; subjects: string[]; onClose: () => void }): JSX.Element {
  const app = useApp()
  const t = app.t
  const [triggerId, setTriggerId] = createSignal(props.triggers[0]?.id ?? '')
  const trigger = createMemo(() => props.triggers.find((x) => x.id === triggerId()))
  const fields = createMemo(() => inputFields(trigger()?.schema))
  const [values, setValues] = createSignal<Record<string, string | boolean>>({})
  const [fieldErrors, setFieldErrors] = createSignal<Record<string, string>>({})
  const [entity, setEntity] = createSignal('')
  const [subjectKey, setSubjectKey] = createSignal('')
  const [subjectOptions, setSubjectOptions] = createSignal<{ key: string; label: string }[]>([])
  const [busy, setBusy] = createSignal(false)
  const [error, setError] = createSignal<string>()
  const set = (name: string, v: string | boolean) => setValues((cur) => ({ ...cur, [name]: v }))

  let searchSeq = 0
  const search = async (query: string) => {
    const mine = ++searchSeq
    if (!entity() || query.trim().length < 1) { setSubjectOptions([]); return }
    try {
      const list = await app.call('automations.search_subjects', { entity: entity(), query: query.trim(), limit: 10 })
      if (mine === searchSeq) setSubjectOptions(list.map((s) => ({ key: s.subject.key, label: s.label })))
    } catch {
      if (mine === searchSeq) setSubjectOptions([])
    }
  }

  const start = async () => {
    if (busy()) return
    const { input, errors } = inputValue(fields(), values())
    const messages: Record<string, string> = {}
    for (const [k, v] of Object.entries(errors)) messages[k] = t(`Run.Field.${v === 'required' ? 'Required' : v === 'number' ? 'Number' : 'Json'}`)
    setFieldErrors(messages)
    if (Object.keys(messages).length > 0) return
    setBusy(true)
    setError(undefined)
    try {
      const res = await app.call('automations.run_manual', {
        id: props.id, trigger_id: triggerId(), input,
        ...(entity() && subjectKey() ? { subject: { entity: entity(), key: subjectKey() } } : {}),
      })
      if (res.ok && res.run_id) {
        const run = res.run_id
        app.toast.notify({ variant: 'success', title: t('Run.Started'), action: { label: t('Run.Open'), onAction: () => app.navigate(graphPath(app.props.basePath, props.id, { run })) } })
        props.onClose()
        return
      }
      if (res.error === 'not_enabled') setError(t('Errors.NotEnabled'))
      else setError(res.fields?.input ? t('Run.InvalidInput', { reason: res.fields.input }) : t('Errors.InvalidInput'))
    } catch (e) {
      setError(errorText(t, e, t('Actions.Run')))
    } finally {
      setBusy(false)
    }
  }

  const field = (f: ReturnType<typeof fields>[number]): JSX.Element => {
    const err = () => fieldErrors()[f.name]
    const label = <>{f.title}{f.required ? ' *' : ''}</>
    return (
      <Switch>
        <Match when={f.kind === 'boolean'}>
          <Toggle label={f.title} checked={Boolean(values()[f.name])} onChange={(e) => set(f.name, e.currentTarget.checked)} data-field={f.name} />
        </Match>
        <Match when={f.kind === 'enum'}>
          <Select label={label} description={f.description} error={err()} placeholder={t('Run.Choose')} value={String(values()[f.name] ?? '')}
            options={(f.options ?? []).map((o, i) => ({ value: String(i), label: o }))} onChange={(e) => set(f.name, e.currentTarget.value)} data-field={f.name} />
        </Match>
        <Match when={f.kind === 'textarea' || f.kind === 'json'}>
          <Textarea label={label} description={f.kind === 'json' ? t('Run.JsonHint') : f.description} error={err()} value={String(values()[f.name] ?? '')}
            onInput={(e) => set(f.name, e.currentTarget.value)} rows={4} data-field={f.name} />
        </Match>
        <Match when={true}>
          <Input type={f.kind === 'number' || f.kind === 'integer' ? 'number' : 'text'} label={label} description={f.description} error={err()}
            value={String(values()[f.name] ?? '')} onInput={(e) => set(f.name, e.currentTarget.value)} data-field={f.name} />
        </Match>
      </Switch>
    )
  }

  return (
    <Dialog open busy={busy()} onOpenChange={(open) => { if (!open) props.onClose() }} class="md:w-full md:max-w-lg" aria-labelledby="run-title" data-testid="run-dialog">
      <form onSubmit={(e) => { e.preventDefault(); void start() }}>
        <header class="flex items-center justify-between gap-3 border-b border-subtle py-1 pl-4 pr-2">
          <h3 id="run-title" class="font-medium" data-dialog-title tabindex="-1">{t('Run.Title')}</h3>
          <button type="button" class="drawer-close" aria-label={t('Common.Close')} data-dialog-close onClick={props.onClose}><Icon name="x" size={20} /></button>
        </header>
        <div class="flex max-h-[65vh] flex-col gap-4 overflow-y-auto px-4 py-3" data-dialog-body>
          <Show when={props.triggers.length > 1}>
            <Select label={t('Run.Trigger')} value={triggerId()} options={props.triggers.map((x) => ({ value: x.id, label: x.title }))}
              onChange={(e) => { setTriggerId(e.currentTarget.value); setValues({}); setFieldErrors({}) }} />
          </Show>
          <Show when={fields().length > 0} fallback={<p class="text-sm text-300">{t('Run.NoInput')}</p>}>
            <For each={fields()}>{(f) => field(f)}</For>
          </Show>
          <Show when={props.subjects.length > 0}>
            <fieldset class="flex flex-col gap-2">
              <legend class="text-sm font-medium">{t('Run.Subject')}</legend>
              <Select value={entity()} placeholder={t('Run.NoSubject')} aria-label={t('Run.SubjectEntity')}
                options={[{ value: '', label: t('Run.NoSubject') }, ...props.subjects.map((s) => ({ value: s, label: s }))]}
                onChange={(e) => { setEntity(e.currentTarget.value); setSubjectKey(''); setSubjectOptions([]) }} />
              <Show when={entity()}>
                <Input type="text" aria-label={t('Run.SubjectSearch')} placeholder={t('Run.SubjectSearch')} onInput={(e) => void search(e.currentTarget.value)} />
                <Show when={subjectOptions().length > 0}>
                  <Select value={subjectKey()} aria-label={t('Run.SubjectPick')} placeholder={t('Run.SubjectPick')}
                    options={subjectOptions().map((o) => ({ value: o.key, label: o.label }))} onChange={(e) => setSubjectKey(e.currentTarget.value)} />
                </Show>
              </Show>
            </fieldset>
          </Show>
          <Show when={error()}><p role="alert" class="text-sm text-red-600" data-testid="run-error">{error()}</p></Show>
        </div>
        <footer class="border-t border-subtle px-4 py-3 pb-[max(0.75rem,env(safe-area-inset-bottom))]">
          <DialogBusyStatus class="mb-2" />
          <menu class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
            <Button type="button" variant="secondary" class="min-h-11 justify-center" data-dialog-close onClick={props.onClose} aria-disabled={busy() || undefined}>{t('Common.Cancel')}</Button>
            <Button type="submit" variant="primary" class="min-h-11 justify-center" loading={busy()} data-testid="run-start" icon={<Icon name="play" />}>
              {busy() ? t('Run.Starting') : t('Run.Start')}
            </Button>
          </menu>
        </footer>
      </form>
    </Dialog>
  )
}
