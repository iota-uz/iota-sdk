import { For, Show, createSignal, type Component } from 'solid-js'
import { describeError, formatTime, shortHash, useConfig } from './context'
import { createLoader } from './load'
import type { WorkflowHistory, WorkflowInstance } from './types'

const STATUSES = ['', 'running', 'waiting', 'blocked', 'completed', 'failed', 'terminated']

/** WorkflowsTab lists workflow instances with their history and offers terminate, start v2, signal, decide, resolve, retry and resume. */
export const WorkflowsTab: Component<{ namespace: string }> = (props) => {
  const cfg = useConfig()
  const { t, rights, api } = cfg
  const [status, setStatus] = createSignal('')
  const [selected, setSelected] = createSignal<string>()
  const [signal, setSignal] = createSignal({ name: '', payload: '' })
  const [busy, setBusy] = createSignal(false)
  const list = createLoader((s) => api.listWorkflows(status(), s))
  const history = createLoader<WorkflowHistory | undefined>(async (s) => {
    const id = selected()
    return id ? api.getWorkflow(id, s) : undefined
  })

  const pick = (id: string) => { setSelected(id); void history.reload() }
  const refresh = async () => { await Promise.all([list.reload(), history.reload()]) }
  const act = async (fn: () => Promise<unknown>, done: string) => {
    setBusy(true)
    try {
      await fn()
      cfg.notify('info', done)
      await refresh()
    } catch (err) {
      cfg.notify('error', describeError(err))
    } finally {
      setBusy(false)
    }
  }
  const json = (text: string): unknown => (text.trim() === '' ? null : JSON.parse(text))
  const mine = (i: WorkflowInstance) => i.Package === props.namespace
  const h = () => history.data()

  return (
    <div>
      <div class="row">
        <h3>{t('Workflows')}</h3>
        <select value={status()} onChange={(e) => { setStatus(e.currentTarget.value); void list.reload() }} aria-label={t('Status')}>
          <For each={STATUSES}>{(s) => <option value={s}>{s || t('Filter')}</option>}</For>
        </select>
        <button type="button" onClick={() => void refresh()}>{t('Refresh')}</button>
      </div>
      <Show when={list.error()}><p class="error" role="alert">{list.error()?.message}</p></Show>
      <Show when={(list.data() ?? []).filter(mine).length > 0} fallback={<p class="muted">{t('NoWorkflows')}</p>}>
        <table data-testid="workflows">
          <thead><tr><th>{t('Workflow')}</th><th>{t('Version')}</th><th>{t('Status')}</th><th>{t('Started')}</th></tr></thead>
          <tbody>
            <For each={(list.data() ?? []).filter(mine)}>{(i) => (
              <tr style={{ cursor: 'pointer', 'font-weight': selected() === i.ID ? '600' : '400' }} onClick={() => pick(i.ID)}>
                <td class="mono">{i.Workflow}</td><td>{i.Version}</td>
                <td classList={{ error: i.Status === 'failed' || i.Status === 'blocked' }}>{i.Status}<Show when={i.BlockedReason}> · {i.BlockedReason}</Show></td>
                <td>{formatTime(i.CreatedAt, cfg.host.locale.language)}</td>
              </tr>
            )}</For>
          </tbody>
        </table>
      </Show>

      <Show when={h()}>{(hist) => (
        <div class="iota-cfg-card" style={{ 'margin-top': '12px' }} data-testid="workflow-history">
          <div class="row">
            <h3>{t('History')}: <span class="mono">{shortHash(hist().Instance.ID)}</span></h3>
            <span class="iota-cfg-badge">{hist().Instance.Status}</span>
            <span class="grow" />
            <Show when={rights.operate}>
              <button type="button" class="danger" disabled={busy()} onClick={() => void act(() => api.terminateWorkflow(hist().Instance.ID), t('Terminate'))}>{t('Terminate')}</button>
              <button type="button" disabled={busy()} onClick={() => void act(() => api.startWorkflowV2(hist().Instance.ID), t('StartV2'))}>{t('StartV2')}</button>
              <button type="button" disabled={busy()} onClick={() => void act(() => api.resumeWorkflow(hist().Instance.ID), t('Resume'))}>{t('Resume')}</button>
            </Show>
          </div>
          <Show when={hist().Instance.BlockedDetail}><p class="error">{hist().Instance.BlockedDetail}</p></Show>

          <h3>{t('Occurrences')}</h3>
          <table>
            <thead><tr><th>#</th><th>{t('Status')}</th><th /></tr></thead>
            <tbody>
              <For each={hist().Occurrences ?? []}>{(o) => (
                <tr>
                  <td class="mono">{o.Name}[{o.N}] <span class="muted">{o.Kind}</span></td>
                  <td classList={{ error: o.Status === 'blocked' || o.Status === 'failed' }}>{o.Status}</td>
                  <td class="row">
                    <Show when={rights.operate && (o.Status === 'blocked' || o.Status === 'failed')}>
                      <button type="button" disabled={busy()} onClick={() => void act(() => api.retryOccurrence(hist().Instance.ID, o.Name, o.N), t('RetryStep'))}>{t('RetryStep')}</button>
                      <button type="button" disabled={busy()} onClick={() => {
                        const raw = window.prompt(t('Payload'), 'null')
                        if (raw !== null) void act(async () => api.resolveOccurrence(hist().Instance.ID, o.Name, o.N, json(raw)), t('Resolve'))
                      }}>{t('Resolve')}</button>
                    </Show>
                  </td>
                </tr>
              )}</For>
            </tbody>
          </table>

          <h3>{t('Tasks')}</h3>
          <ul>
            <For each={hist().Tasks ?? []}>{(task) => (
              <li>
                <span class="mono">{task.Name}</span> · {task.Status} · <span class="mono" data-testid="task-assignee">{task.AssigneeKind === 'permission' ? task.AssigneePermission : `#${task.AssigneeUserID}`}</span>
                <Show when={task.Status === 'open' || task.Status === 'pending'}>
                  <button type="button" disabled={busy()} onClick={() => {
                    const raw = window.prompt(t('Decision'), JSON.stringify(task.DecisionSchema ?? null))
                    if (raw !== null) void act(async () => api.decideTask(task.ID, json(raw)), t('Decide'))
                  }}>{t('Decide')}</button>
                </Show>
              </li>
            )}</For>
          </ul>

          <h3>{t('Signals')}</h3>
          <ul>
            <For each={hist().Signals ?? []}>{(s) => <li><span class="mono">{s.Name}</span> · {formatTime(s.CreatedAt, cfg.host.locale.language)}{s.ConsumedAt ? ' ✓' : ''}</li>}</For>
          </ul>
          <Show when={rights.operate}>
            <div class="row">
              <input type="text" placeholder={t('SignalName')} value={signal().name} onInput={(e) => setSignal({ ...signal(), name: e.currentTarget.value })} />
              <input type="text" placeholder={t('Payload')} value={signal().payload} onInput={(e) => setSignal({ ...signal(), payload: e.currentTarget.value })} />
              <button type="button" disabled={busy() || !signal().name} onClick={() => void act(async () => api.signalWorkflow(hist().Instance.ID, signal().name, json(signal().payload)), t('Signal'))}>{t('Signal')}</button>
            </div>
          </Show>

          <h3>{t('Details')}</h3>
          <ul class="mono">
            <For each={hist().Invocations ?? []}>{(inv) => <li>{inv.Contract} · {inv.BindingPackage}@{inv.BindingVersion} · {inv.Outcome}</li>}</For>
          </ul>
        </div>
      )}</Show>
    </div>
  )
}
