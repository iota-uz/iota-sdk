import { useCallback, useEffect, useRef, useState } from 'react'
import { BookmarkSimple } from '../icons'
import type { DashboardDocument } from '../contract'
import { cubeFilterParam, cubeGroupByParam, filterParamNames, useDashboard, useTranslate } from '../runtime'

interface SavedView {
  id: string
  dashboardId: string
  name: string
  scope: 'personal' | 'team'
  stateUrl: string
  defaultRoleId?: number
}

interface ExportSchedule {
  id: string
  viewId: string
  name: string
  cron: string
  timezone: string
  recipients: string[]
  nextRunAt?: string
}

interface ShareCapabilities {
  manageTeam: boolean
  scheduleMail: boolean
  roles?: Array<{ id: number; name: string }>
}

const navigationStateParams = [
  'path', 'perspective', 'drawer', 'drawerPath', 'drawerPerspective', 'drawerPanel', 'lensHidden',
]

// eslint-disable-next-line react-refresh/only-export-components
export function roleDefaultURL(
  document_: DashboardDocument,
  current: URL,
  defaultView: SavedView | undefined,
  storage: Pick<Storage, 'getItem' | 'setItem'>,
): string | undefined {
  if (!defaultView) return undefined
  const stateParams = new Set([
    ...navigationStateParams,
    cubeFilterParam,
    cubeGroupByParam,
    ...filterParamNames(document_),
  ])
  if ([...current.searchParams.keys()].some((name) => stateParams.has(name))) return undefined
  let target: URL
  try {
    target = new URL(defaultView.stateUrl, current)
  } catch {
    return undefined
  }
  if (target.origin !== current.origin) return undefined
  const relative = `${target.pathname}${target.search}${target.hash}`
  if (relative === `${current.pathname}${current.search}${current.hash}`) return undefined
  const guard = `lens:role-default:${document_.meta.dashboardId}:${defaultView.id}`
  try {
    if (storage.getItem(guard) === relative) return undefined
    storage.setItem(guard, relative)
  } catch {
    // Storage can be disabled by browser policy. The URL-state check still
    // prevents loops for normal saved slices, so default application remains
    // useful without making storage availability a hard requirement.
  }
  return relative
}

function csrfToken(node?: Node | null): string | undefined {
  const root = node?.getRootNode()
  const hostToken = typeof ShadowRoot !== 'undefined' && root instanceof ShadowRoot
    ? root.host.getAttribute('csrf')
    : undefined
  return hostToken || document.querySelector<HTMLMetaElement>('meta[name="csrf-token"], meta[name="csrf"]')?.content || undefined
}

async function requestJSON<T>(endpoint: string, token?: string, init?: RequestInit): Promise<T> {
  const target = endpoint.trim()
  if (!target.startsWith('/') || target.startsWith('//') || target.includes('\\')) {
    throw new Error('sharing endpoint must be site-relative')
  }
  const response = await fetch(target, {
    credentials: 'same-origin',
    ...init,
    headers: {
      Accept: 'application/json',
      ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
      ...(token ? { 'X-CSRF-Token': token } : {}),
      ...init?.headers,
    },
  })
  if (!response.ok) {
    const body = await response.json().catch(() => ({})) as { error?: string }
    throw new Error(body.error || `request failed (${response.status})`)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export function SavedViewsMenu() {
  const { document: document_ } = useDashboard()
  const documentRef = useRef(document_)
  documentRef.current = document_
  const translate = useTranslate()
  const translateRef = useRef(translate)
  translateRef.current = translate
  const endpoints = document_.endpoints
  const [open, setOpen] = useState(false)
  const [views, setViews] = useState<SavedView[]>([])
  const [schedules, setSchedules] = useState<ExportSchedule[]>([])
  const [name, setName] = useState('')
  const [scope, setScope] = useState<'personal' | 'team'>('personal')
  const [defaultRole, setDefaultRole] = useState('')
  const [scheduleView, setScheduleView] = useState('')
  const [recipients, setRecipients] = useState('')
  const [cron, setCron] = useState('0 8 * * 1')
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string>()
  const [capabilities, setCapabilities] = useState<ShareCapabilities>({ manageTeam: false, scheduleMail: false })
  const container = useRef<HTMLDivElement>(null)
  const dialog = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const dashboardID = document_.meta.dashboardId

  const load = useCallback(async () => {
    if (!endpoints.views) return
    setPending(true)
    setError(undefined)
    try {
      const token = csrfToken(container.current)
      const viewResult = await requestJSON<{ views: SavedView[]; defaultView?: SavedView; capabilities?: ShareCapabilities }>(
        `${endpoints.views}?dashboard=${encodeURIComponent(dashboardID)}`,
        token,
      )
      const resolvedCapabilities = viewResult.capabilities ?? { manageTeam: false, scheduleMail: false }
      const scheduleResult = endpoints.schedules && resolvedCapabilities.scheduleMail
        ? await requestJSON<{ schedules: ExportSchedule[] }>(
            `${endpoints.schedules}?dashboard=${encodeURIComponent(dashboardID)}`,
            token,
          )
        : { schedules: [] }
      setViews(viewResult.views)
      setCapabilities(resolvedCapabilities)
      setSchedules(scheduleResult.schedules)
      setScheduleView((current) => (
        viewResult.views.some(({ id }) => id === current) ? current : viewResult.views[0]?.id || ''
      ))
      if (typeof window !== 'undefined') {
        const target = roleDefaultURL(documentRef.current, new URL(window.location.href), viewResult.defaultView, window.sessionStorage)
        if (target) window.location.replace(target)
      }
    } catch (cause: unknown) {
      setError(cause instanceof Error ? cause.message : translateRef.current('views.error', 'Saved views could not be loaded'))
    } finally {
      setPending(false)
    }
  }, [dashboardID, endpoints.schedules, endpoints.views])

  useEffect(() => { void load() }, [load])

  useEffect(() => {
    if (!open) return undefined
    const focusFrame = requestAnimationFrame(() => dialog.current?.focus())
    const close = (event: PointerEvent) => {
      if (event.target instanceof Node && !container.current?.contains(event.target)) setOpen(false)
    }
    const escape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      setOpen(false)
      trigger.current?.focus()
    }
    document.addEventListener('pointerdown', close, true)
    document.addEventListener('keydown', escape, true)
    return () => {
      cancelAnimationFrame(focusFrame)
      document.removeEventListener('pointerdown', close, true)
      document.removeEventListener('keydown', escape, true)
    }
  }, [open])

  const currentStateURL = typeof window === 'undefined'
    ? '/'
    : `${window.location.pathname}${window.location.search}${window.location.hash}`

  if (!endpoints.views) return null

  const save = async () => {
    const trimmed = name.trim()
    if (!trimmed) return
    setPending(true)
    setError(undefined)
    try {
      await requestJSON<SavedView>(endpoints.views!, csrfToken(container.current), {
        method: 'POST',
        body: JSON.stringify({
          dashboardId: dashboardID,
          name: trimmed,
          scope,
          stateUrl: currentStateURL,
          ...(scope === 'team' && Number(defaultRole) > 0 ? { defaultRoleId: Number(defaultRole) } : {}),
        }),
      })
      setName('')
      await load()
    } catch (cause: unknown) {
      setError(cause instanceof Error ? cause.message : translate('views.error', 'Saved view could not be saved'))
      setPending(false)
    }
  }

  const removeView = async (id: string) => {
    setPending(true)
    try {
      await requestJSON<void>(`${endpoints.views}/${encodeURIComponent(id)}`, csrfToken(container.current), { method: 'DELETE' })
      await load()
    } catch (cause: unknown) {
      setError(cause instanceof Error ? cause.message : translate('views.error', 'Saved view could not be removed'))
      setPending(false)
    }
  }

  const createSchedule = async () => {
    if (!endpoints.schedules || !scheduleView || !recipients.trim()) return
    setPending(true)
    setError(undefined)
    try {
      await requestJSON<ExportSchedule>(endpoints.schedules, csrfToken(container.current), {
        method: 'POST',
        body: JSON.stringify({
          dashboardId: dashboardID,
          viewId: scheduleView,
          name: views.find(({ id }) => id === scheduleView)?.name || document_.meta.title,
          cron,
          timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
          recipients: recipients.split(',').map((value) => value.trim()).filter(Boolean),
          enabled: true,
        }),
      })
      setRecipients('')
      await load()
    } catch (cause: unknown) {
      setError(cause instanceof Error ? cause.message : translate('views.error', 'Schedule could not be saved'))
      setPending(false)
    }
  }

  const removeSchedule = async (id: string) => {
    if (!endpoints.schedules) return
    setPending(true)
    try {
      await requestJSON<void>(`${endpoints.schedules}/${encodeURIComponent(id)}`, csrfToken(container.current), { method: 'DELETE' })
      await load()
    } catch (cause: unknown) {
      setError(cause instanceof Error ? cause.message : translate('views.error', 'Schedule could not be removed'))
      setPending(false)
    }
  }

  return (
    <div className="lens-saved-views" ref={container}>
      <button
        aria-expanded={open}
        aria-haspopup="dialog"
        className="lens-export-button"
        onClick={() => setOpen((current) => !current)}
        ref={trigger}
        type="button"
      >
        <BookmarkSimple />
        <span>{translate('views.title', 'Views')}</span>
      </button>
      {open && (
        <div aria-label={translate('views.title', 'Saved views')} className="lens-saved-views-popover" ref={dialog} role="dialog" tabIndex={-1}>
          <div className="lens-saved-views-section">
            <strong>{translate('views.saved', 'Saved views')}</strong>
            {views.length === 0 && !pending && <span className="lens-muted">{translate('views.empty', 'No saved views')}</span>}
            {views.map((view) => (
              <div className="lens-saved-view-row" key={view.id}>
                <a className="lens-saved-view-open" href={view.stateUrl}>
                  <span>{view.name}</span>
                  <small>{view.scope === 'team' ? translate('views.team', 'Team') : translate('views.personal', 'Personal')}</small>
                </a>
                {(view.scope === 'personal' || capabilities.manageTeam) && (
                  <button aria-label={translate('views.delete', 'Delete {name}', { name: view.name })} className="lens-saved-view-delete" onClick={() => { void removeView(view.id) }} type="button">×</button>
                )}
              </div>
            ))}
          </div>
          <div className="lens-saved-views-section">
            <strong>{translate('views.saveCurrent', 'Save current slice')}</strong>
            <input aria-label={translate('views.name', 'View name')} onChange={(event) => setName(event.target.value)} placeholder={translate('views.name', 'View name')} value={name} />
            <div className="lens-saved-view-fields">
              <select aria-label={translate('views.scope', 'Visibility')} onChange={(event) => setScope(event.target.value as 'personal' | 'team')} value={scope}>
                <option value="personal">{translate('views.personal', 'Personal')}</option>
                {capabilities.manageTeam && <option value="team">{translate('views.team', 'Team')}</option>}
              </select>
              {scope === 'team' && (
                <select aria-label={translate('views.defaultRole', 'Default for role')} onChange={(event) => setDefaultRole(event.target.value)} value={defaultRole}>
                  <option value="">{translate('views.noDefaultRole', 'No role default')}</option>
                  {(capabilities.roles ?? []).map((role) => <option key={role.id} value={role.id}>{role.name}</option>)}
                </select>
              )}
              <button disabled={pending || !name.trim()} onClick={() => { void save() }} type="button">{translate('views.save', 'Save')}</button>
            </div>
          </div>
          {endpoints.schedules && capabilities.scheduleMail && views.length > 0 && (
            <div className="lens-saved-views-section">
              <strong>{translate('views.schedule', 'Email schedule')}</strong>
              <select aria-label={translate('views.saved', 'Saved view')} onChange={(event) => setScheduleView(event.target.value)} value={scheduleView}>
                {views.map((view) => <option key={view.id} value={view.id}>{view.name}</option>)}
              </select>
              <input aria-label={translate('views.recipients', 'Recipients')} onChange={(event) => setRecipients(event.target.value)} placeholder={translate('views.recipientsHint', 'email@example.com, team@example.com')} value={recipients} />
              <div className="lens-saved-view-fields">
                <input aria-label={translate('views.cron', 'Cron schedule')} onChange={(event) => setCron(event.target.value)} value={cron} />
                <button disabled={pending || !recipients.trim()} onClick={() => { void createSchedule() }} type="button">{translate('views.scheduleSave', 'Schedule XLSX')}</button>
              </div>
              {schedules.map((schedule) => (
                <div className="lens-saved-view-row" key={schedule.id}>
                  <small className="lens-saved-view-schedule">{schedule.name} · {schedule.cron} · {schedule.recipients.join(', ')}</small>
                  <button aria-label={translate('views.delete', 'Delete {name}', { name: schedule.name })} className="lens-saved-view-delete" onClick={() => { void removeSchedule(schedule.id) }} type="button">×</button>
                </div>
              ))}
            </div>
          )}
          {pending && <span className="lens-muted" role="status">{translate('views.loading', 'Loading…')}</span>}
          {error && <span className="lens-export-message lens-export-message-error" role="alert">{error}</span>}
        </div>
      )}
    </div>
  )
}
