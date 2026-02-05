import { useEffect, useMemo, useState } from 'react'
import { useAppletContext } from '../applet-core/context/AppletContext'
import { useAppletRuntime } from '../applet-core/hooks/useAppletRuntime'

type RPCEvent = {
  id: string
  method: string
  status: 'start' | 'success' | 'error'
  durationMs?: number
  error?: unknown
}

export function AppletDevtoolsOverlay() {
  const ctx = useAppletContext()
  const runtime = useAppletRuntime()
  const [rpcEvents, setRPCEvents] = useState<RPCEvent[]>([])

  useEffect(() => {
    const onEvent = (e: Event) => {
      const ce = e as CustomEvent<RPCEvent>
      setRPCEvents((prev) => [ce.detail, ...prev].slice(0, 50))
    }
    window.addEventListener('iota:applet-rpc', onEvent as EventListener)
    return () => window.removeEventListener('iota:applet-rpc', onEvent as EventListener)
  }, [])

  const summary = useMemo(() => {
    return {
      basePath: runtime.basePath,
      assetsBasePath: runtime.assetsBasePath,
      rpcEndpoint: runtime.rpcEndpoint,
      shellMode: runtime.shellMode,
      route: ctx.route,
      user: { id: ctx.user.id, email: ctx.user.email },
      tenant: ctx.tenant,
    }
  }, [ctx.route, ctx.tenant, ctx.user.email, ctx.user.id, runtime.assetsBasePath, runtime.basePath, runtime.rpcEndpoint, runtime.shellMode])

  return (
    <div
      style={{
        position: 'fixed',
        right: 12,
        bottom: 12,
        width: 420,
        maxHeight: '60vh',
        overflow: 'auto',
        background: 'rgba(17, 24, 39, 0.92)',
        color: '#E5E7EB',
        border: '1px solid rgba(255,255,255,0.12)',
        borderRadius: 10,
        padding: 12,
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
        fontSize: 12,
        zIndex: 2147483647,
      }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
        <div style={{ fontWeight: 700 }}>Applet Devtools</div>
        <div style={{ opacity: 0.7 }}>{runtime.shellMode ?? 'unknown'}</div>
      </div>

      <div style={{ marginBottom: 10 }}>
        <div style={{ opacity: 0.85, marginBottom: 4 }}>Context</div>
        <pre style={{ margin: 0, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>{JSON.stringify(summary, null, 2)}</pre>
      </div>

      <div>
        <div style={{ opacity: 0.85, marginBottom: 4 }}>RPC</div>
        {rpcEvents.length === 0 ? (
          <div style={{ opacity: 0.7 }}>No calls yet</div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {rpcEvents.map((ev) => (
              <div key={`${ev.id}:${ev.status}`} style={{ padding: 8, border: '1px solid rgba(255,255,255,0.08)', borderRadius: 8 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                  <div>{ev.method}</div>
                  <div style={{ opacity: 0.8 }}>
                    {ev.status}
                    {typeof ev.durationMs === 'number' ? ` (${ev.durationMs}ms)` : ''}
                  </div>
                </div>
                {ev.status === 'error' ? (
                  <pre style={{ margin: '6px 0 0', opacity: 0.8, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                    {JSON.stringify(ev.error ?? {}, null, 2)}
                  </pre>
                ) : null}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
