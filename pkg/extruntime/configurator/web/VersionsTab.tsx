import { For, Show, createSignal, type Component } from 'solid-js'
import { describeError, formatTime, shortHash, useConfig } from './context'
import { createLoader } from './load'
import type { PackageInfo, VersionInfo } from './types'

export interface VersionsTabProps {
  pkg: PackageInfo
  /** Bumped by the parent after a publish. */
  refreshKey: number
  onChanged(): void
}

/** VersionsTab lists published versions with their hashes and runs review, activation and binding actions. */
export const VersionsTab: Component<VersionsTabProps> = (props) => {
  const cfg = useConfig()
  const { t, rights, api } = cfg
  const ns = props.pkg.namespace
  const versions = createLoader((s) => { void props.refreshKey; return api.listVersions(ns, s) })
  const bindings = createLoader((s) => api.listBindings(s))
  const owners = createLoader((s) => api.listOwners(s))
  const [comment, setComment] = createSignal('')
  const [busy, setBusy] = createSignal(false)
  const [bind, setBind] = createSignal({ slotKind: 'command', targetId: '', version: '', export: '' })
  const [owner, setOwner] = createSignal({ triggerId: '', userId: '' })

  const act = async (fn: () => Promise<unknown>, done?: string) => {
    setBusy(true)
    try {
      await fn()
      if (done) cfg.notify('info', done)
      await Promise.all([versions.reload(), bindings.reload(), owners.reload()])
      props.onChanged()
    } catch (err) {
      cfg.notify('error', describeError(err))
    } finally {
      setBusy(false)
    }
  }

  const mine = (b: { package: string }) => b.package === ns

  return (
    <div>
      <h3>{t('Versions')}</h3>
      <Show when={versions.error()}><p class="error" role="alert">{versions.error()?.message} <button type="button" onClick={() => void versions.reload()}>{t('Retry')}</button></p></Show>
      <Show when={(versions.data() ?? []).length > 0} fallback={<p class="muted">{t('NoVersions')}</p>}>
        <table data-testid="versions">
          <thead><tr>
            <th>{t('Version')}</th><th>{t('SourceHash')}</th><th>{t('ArtifactHash')}</th><th>{t('Status')}</th><th>{t('Started')}</th><th />
          </tr></thead>
          <tbody>
            <For each={versions.data()}>{(v: VersionInfo) => (
              <tr>
                <td><strong>{v.version}</strong> <Show when={v.active}><span class="iota-cfg-badge ok">{t('Active')}</span></Show></td>
                <td class="mono" title={v.sourceHash}>{shortHash(v.sourceHash)}</td>
                <td class="mono" title={v.artifactHash}>{shortHash(v.artifactHash)}</td>
                <td>
                  <span classList={{ ok: v.approved, warning: !v.approved }}>{v.approved ? t('Approved') : t('NotApproved')}</span>
                  <Show when={v.latestReview}>{(r) => <div class="muted">{r().comment} · {shortHash(r().sourceHash)}</div>}</Show>
                </td>
                <td>{formatTime(v.createdAt, cfg.host.locale.language)}</td>
                <td class="row">
                  <Show when={rights.review}>
                    <button type="button" disabled={busy()} onClick={() => void act(() => api.review(ns, v.version, true, comment(), v))}>{t('Approve')}</button>
                    <button type="button" class="danger" disabled={busy()} onClick={() => void act(() => api.review(ns, v.version, false, comment(), v))}>{t('Reject')}</button>
                  </Show>
                  <Show when={rights.publish && !v.active}>
                    <button type="button" class="primary" disabled={busy() || !v.approved} onClick={() => void act(() => api.activate(ns, v.version), t('Activate'))}>{t('Activate')}</button>
                  </Show>
                  <Show when={rights.publish && v.active}>
                    <button type="button" class="danger" disabled={busy()} onClick={() => void act(() => api.deactivate(ns), t('Deactivate'))}>{t('Deactivate')}</button>
                  </Show>
                  <a href={api.exportUrl(ns, v.version)} download="">{t('Export')}</a>
                </td>
              </tr>
            )}</For>
          </tbody>
        </table>
        <Show when={rights.review}>
          <div class="row" style={{ 'margin-top': '6px' }}>
            <label>{t('Comment')} <input type="text" value={comment()} onInput={(e) => setComment(e.currentTarget.value)} size={40} /></label>
          </div>
        </Show>
      </Show>

      <h3 style={{ 'margin-top': '16px' }}>{t('Bindings')}</h3>
      <table data-testid="bindings">
        <thead><tr><th>{t('SlotKind')}</th><th>{t('TargetID')}</th><th>{t('Version')}</th><th /></tr></thead>
        <tbody>
          <For each={bindings.data() ?? []}>{(b) => (
            <tr>
              <td>{b.slotKind}</td><td class="mono">{b.targetId}</td><td>{b.package}@{b.version}#{b.export}</td>
              <td><Show when={rights.publish}><button type="button" class="danger" disabled={busy()} onClick={() => void act(() => api.unbind(b.slotKind, b.targetId), t('Unbind'))}>{t('Unbind')}</button></Show></td>
            </tr>
          )}</For>
        </tbody>
      </table>
      <Show when={rights.publish}>
        <div class="row" style={{ 'margin-top': '6px' }}>
          <select value={bind().slotKind} onChange={(e) => setBind({ ...bind(), slotKind: e.currentTarget.value })}>
            <option value="command">command</option><option value="screen">screen</option><option value="shell">shell</option>
          </select>
          <input type="text" placeholder={t('TargetID')} value={bind().targetId} onInput={(e) => setBind({ ...bind(), targetId: e.currentTarget.value })} />
          <select value={bind().version} onChange={(e) => setBind({ ...bind(), version: e.currentTarget.value })}>
            <option value="">{t('Version')}</option>
            <For each={(versions.data() ?? []).filter((v) => v.active)}>{(v) => <option value={v.version}>{v.version}</option>}</For>
          </select>
          <input type="text" placeholder="export" value={bind().export} onInput={(e) => setBind({ ...bind(), export: e.currentTarget.value })} />
          <button type="button" disabled={busy() || !bind().targetId || !bind().version || !bind().export}
            onClick={() => void act(() => api.bind(bind().slotKind, bind().targetId, ns, bind().version, bind().export), t('Bind'))}>{t('Bind')}</button>
        </div>
      </Show>
      <Show when={(bindings.data() ?? []).some(mine)}><p class="muted">{ns}</p></Show>

      <h3 style={{ 'margin-top': '16px' }}>{t('Owners')}</h3>
      <table data-testid="owners">
        <thead><tr><th>{t('TriggerID')}</th><th>{t('UserID')}</th><th /></tr></thead>
        <tbody>
          <For each={(owners.data() ?? []).filter((o) => o.triggerId.startsWith(ns + '.'))}>{(o) => (
            <tr>
              <td class="mono">{o.triggerId}</td><td>{o.userId}</td>
              <td><Show when={rights.publish}><button type="button" class="danger" disabled={busy()} onClick={() => void act(() => api.removeOwner(o.triggerId), t('Remove'))}>{t('Remove')}</button></Show></td>
            </tr>
          )}</For>
        </tbody>
      </table>
      <Show when={rights.publish}>
        <div class="row" style={{ 'margin-top': '6px' }}>
          <input type="text" placeholder={t('TriggerID')} value={owner().triggerId} onInput={(e) => setOwner({ ...owner(), triggerId: e.currentTarget.value })} />
          <input type="number" placeholder={t('UserID')} value={owner().userId} onInput={(e) => setOwner({ ...owner(), userId: e.currentTarget.value })} />
          <button type="button" disabled={busy() || !owner().triggerId || !Number(owner().userId)}
            onClick={() => void act(() => api.assignOwner(owner().triggerId, Number(owner().userId)), t('AssignOwner'))}>{t('AssignOwner')}</button>
        </div>
      </Show>
    </div>
  )
}
