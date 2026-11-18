import { For, Show, type Component } from 'solid-js'
import { formatTime, shortHash, useConfig } from './context'
import { createLoader } from './load'

/** AuditTab shows the management audit log of the package, including refused attempts. */
export const AuditTab: Component<{ namespace: string; refreshKey: number }> = (props) => {
  const cfg = useConfig()
  const { t, api } = cfg
  const audit = createLoader((s) => { void props.refreshKey; return api.audit(props.namespace, s) })
  return (
    <div>
      <div class="row"><h3>{t('Audit')}</h3><button type="button" onClick={() => void audit.reload()}>{t('Refresh')}</button></div>
      <Show when={audit.error()}><p class="error" role="alert">{audit.error()?.message}</p></Show>
      <Show when={(audit.data() ?? []).length > 0} fallback={<p class="muted">{t('NoAudit')}</p>}>
        <table data-testid="audit">
          <thead><tr><th>{t('Started')}</th><th>{t('Actor')}</th><th>{t('Action')}</th><th>{t('Version')}</th><th>{t('Outcome')}</th><th>{t('Hashes')}</th></tr></thead>
          <tbody>
            <For each={audit.data()}>{(a) => (
              <tr>
                <td>{formatTime(a.at, cfg.host.locale.language)}</td>
                <td>#{a.actorUserId} <span class="muted">{a.credential}</span></td>
                <td class="mono">{a.action} {a.target}</td><td>{a.version}</td>
                <td classList={{ ok: a.outcome === 'ok', error: a.outcome !== 'ok' }}>{a.outcome}</td>
                <td class="mono" title={`${a.sourceHash ?? ''} ${a.artifactHash ?? ''}`}>{shortHash(a.sourceHash)} {shortHash(a.artifactHash)}</td>
              </tr>
            )}</For>
          </tbody>
        </table>
      </Show>
    </div>
  )
}
