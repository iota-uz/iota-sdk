import { useCallback, useEffect, useId, useLayoutEffect, useRef, useState } from 'react'
import { BookmarkSimple } from '../icons'
import { useDashboard, useTranslate } from '../runtime'
import { useFocusTrap } from '../runtime/focusTrap'

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
  lastError?: string
}

interface ShareCapabilities {
  manageTeam: boolean
  scheduleMail: boolean
  roles?: Array<{ id: number; name: string }>
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

function safeStateURL(raw: string): string | undefined {
  if (!raw.startsWith('/') || raw.startsWith('//') || raw.includes('\\')) return undefined
  const parsed = new URL(raw, window.location.origin)
  if (parsed.origin !== window.location.origin) return undefined
  return `${parsed.pathname}${parsed.search}${parsed.hash}`
}

export function SavedViewsMenu() {
  const { document: document_ } = useDashboard()
  const dialogID = useId()
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
  const [popoverPosition, setPopoverPosition] = useState({ maxHeight: 620, top: 0 })
  const [capabilities, setCapabilities] = useState<ShareCapabilities>({ manageTeam: false, scheduleMail: false })
  const container = useRef<HTMLDivElement>(null)
  const dialog = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const dashboardID = document_.meta.dashboardId
  const closePopover = useCallback(() => setOpen(false), [])
  useFocusTrap(dialog, open, closePopover, undefined, trigger)

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
    } catch (cause: unknown) {
      console.error('[lens] saved views could not be loaded', cause)
      setError(translateRef.current('views.loadError', 'Saved views could not be loaded'))
    } finally {
      setPending(false)
    }
  }, [dashboardID, endpoints.schedules, endpoints.views])

  useEffect(() => { void load() }, [load])

  useEffect(() => {
    if (!open) return undefined
    const close = (event: PointerEvent) => {
      if (event.target instanceof Node && !container.current?.contains(event.target)) setOpen(false)
    }
    document.addEventListener('pointerdown', close, true)
    return () => {
      document.removeEventListener('pointerdown', close, true)
    }
  }, [open])

  useLayoutEffect(() => {
    if (!open) return undefined
    const reposition = () => {
      const bounds = trigger.current?.getBoundingClientRect()
      if (!bounds) return
      const height = dialog.current?.getBoundingClientRect().height ?? 0
      const below = bounds.bottom + 8
      const top = below + height <= window.innerHeight
        ? below
        : Math.max(16, bounds.top - height - 8)
      setPopoverPosition({ maxHeight: Math.max(120, window.innerHeight - top - 16), top })
    }
    reposition()
    const observer = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(reposition)
    if (dialog.current) observer?.observe(dialog.current)
    window.addEventListener('resize', reposition)
    window.addEventListener('scroll', reposition, true)
    return () => {
      observer?.disconnect()
      window.removeEventListener('resize', reposition)
      window.removeEventListener('scroll', reposition, true)
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
      console.error('[lens] saved view could not be saved', cause)
      setError(translate('views.saveError', 'Saved view could not be saved'))
      setPending(false)
    }
  }

  const removeView = async (id: string) => {
    if (!window.confirm(translate('views.confirmDelete', 'Delete this saved view?'))) return
    setPending(true)
    try {
      await requestJSON<void>(`${endpoints.views}/${encodeURIComponent(id)}`, csrfToken(container.current), { method: 'DELETE' })
      await load()
    } catch (cause: unknown) {
      console.error('[lens] saved view could not be removed', cause)
      setError(translate('views.deleteError', 'Saved view could not be removed'))
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
      console.error('[lens] schedule could not be saved', cause)
      setError(translate('views.scheduleSaveError', 'Schedule could not be saved'))
      setPending(false)
    }
  }

  const removeSchedule = async (id: string) => {
    if (!window.confirm(translate('views.confirmDeleteSchedule', 'Delete this schedule?'))) return
    if (!endpoints.schedules) return
    setPending(true)
    try {
      await requestJSON<void>(`${endpoints.schedules}/${encodeURIComponent(id)}`, csrfToken(container.current), { method: 'DELETE' })
      await load()
    } catch (cause: unknown) {
      console.error('[lens] schedule could not be removed', cause)
      setError(translate('views.scheduleDeleteError', 'Schedule could not be removed'))
      setPending(false)
    }
  }

  return (
    <div className="lens-saved-views" ref={container}>
      <button
        aria-controls={dialogID}
        aria-expanded={open}
        aria-haspopup="dialog"
        className="lens-export-button"
        onClick={() => setOpen((current) => !current)}
        ref={trigger}
        type="button"
      >
        <BookmarkSimple />
        <span>{translate('views.triggerTitle', 'Views')}</span>
      </button>
      {open && (
        <div aria-label={translate('views.dialogTitle', 'Saved views')} aria-modal="true" className="lens-saved-views-popover" id={dialogID} ref={dialog} role="dialog" style={popoverPosition} tabIndex={-1}>
          <div className="lens-saved-views-section">
            <strong>{translate('views.savedList', 'Saved views')}</strong>
            {views.length === 0 && !pending && <span className="lens-muted">{translate('views.empty', 'No saved views')}</span>}
            {views.map((view) => (
              <div className="lens-saved-view-row" key={view.id}>
                <a className="lens-saved-view-open" href={safeStateURL(view.stateUrl)}>
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
              <select aria-label={translate('views.savedView', 'Saved view')} onChange={(event) => setScheduleView(event.target.value)} value={scheduleView}>
                {views.map((view) => <option key={view.id} value={view.id}>{view.name}</option>)}
              </select>
              <input aria-label={translate('views.recipients', 'Recipients')} onChange={(event) => setRecipients(event.target.value)} placeholder={translate('views.recipientsHint', 'email@example.com, team@example.com')} value={recipients} />
              <div className="lens-saved-view-fields">
                <input aria-label={translate('views.cron', 'Cron schedule')} onChange={(event) => setCron(event.target.value)} value={cron} />
                <button disabled={pending || !recipients.trim()} onClick={() => { void createSchedule() }} type="button">{translate('views.scheduleSave', 'Schedule XLSX')}</button>
              </div>
              {schedules.map((schedule) => (
                <div className="lens-saved-view-row" key={schedule.id}>
                  <small className="lens-saved-view-schedule">
                    {schedule.name} · {schedule.cron} · {schedule.recipients.join(', ')}
                    {schedule.nextRunAt && <> · {translate('views.nextRun', 'Next run: {time}', { time: schedule.nextRunAt })}</>}
                    {schedule.lastError && <span className="lens-saved-view-error" role="alert">{schedule.lastError}</span>}
                  </small>
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
