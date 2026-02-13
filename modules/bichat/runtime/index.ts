type RuntimeSDK = typeof import('@iota-uz/sdk/applet-runtime')

async function loadRuntimeSDK(): Promise<RuntimeSDK> {
  try {
    return await import('@iota-uz/sdk/applet-runtime')
  } catch {
    return await import('../../../../applets/ui/src/applet-runtime/index.ts')
  }
}

const { auth, db, defineApplet, engine, kv, ws } = await loadRuntimeSDK()

ws.onConnection(async (connectionId) => {
  await kv.set(`ws:connection:${connectionId}`, {
    connectedAt: new Date().toISOString(),
  })
})

ws.onMessage(async (connectionId, data) => {
  const text = Buffer.from(data).toString('utf8')
  await kv.set(`ws:last-message:${connectionId}`, {
    text,
    receivedAt: new Date().toISOString(),
  })
  try {
    await ws.send(connectionId, { type: 'ack', receivedAt: new Date().toISOString() })
  } catch {
    // Ignore ack errors during early bring-up when connection may already be closed.
  }
})

ws.onClose(async (connectionId) => {
  await kv.del(`ws:connection:${connectionId}`)
})

function json(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: {
      'content-type': 'application/json; charset=utf-8',
    },
  })
}

function resolveGoDelegateMethod(method: string): string {
  const trimmed = String(method).trim()
  if (!trimmed.startsWith('bichat.')) {
    return trimmed
  }
  return `bichat.__go.${trimmed.slice('bichat.'.length)}`
}

const bichatPublicMethods = new Set<string>([
  'bichat.ping',
  'bichat.session.list',
  'bichat.session.create',
  'bichat.session.get',
  'bichat.session.updateTitle',
  'bichat.session.clear',
  'bichat.session.compact',
  'bichat.session.delete',
  'bichat.session.pin',
  'bichat.session.unpin',
  'bichat.session.artifacts',
  'bichat.session.uploadArtifacts',
  'bichat.artifact.update',
  'bichat.artifact.delete',
  'bichat.session.archive',
  'bichat.session.unarchive',
  'bichat.session.regenerateTitle',
  'bichat.question.submit',
  'bichat.question.reject',
])

defineApplet({
  async fetch(request) {
    const url = new URL(request.url)

    if (url.pathname === '/__public_rpc' && request.method === 'POST') {
      const payload = (await request.json()) as {
        id?: string | number | null
        method?: string
        params?: unknown
      }
      const method = String(payload.method ?? '').trim()
      if (!method) {
        return json(
          {
            jsonrpc: '2.0',
            id: payload.id ?? null,
            error: { code: -32600, message: 'Invalid Request' },
          },
          200,
        )
      }
      if (!bichatPublicMethods.has(method)) {
        return json(
          {
            jsonrpc: '2.0',
            id: payload.id ?? null,
            error: { code: -32601, message: 'Method not found' },
          },
          200,
        )
      }

      try {
        const result = await engine.call(resolveGoDelegateMethod(method), payload.params ?? {})
        return json({
          jsonrpc: '2.0',
          id: payload.id ?? null,
          result,
        })
      } catch (error) {
        const message = error instanceof Error ? error.message : 'request failed'
        return json(
          {
            jsonrpc: '2.0',
            id: payload.id ?? null,
            error: { code: 'error', message },
          },
          200,
        )
      }
    }

    if (url.pathname === '/__health') {
      return json({ ok: true, applet: process.env.IOTA_APPLET_ID ?? 'unknown' })
    }

    if (url.pathname === '/__probe') {
      const currentUser = await auth.currentUser()
      const probeKey = `probe:${currentUser.tenantId}:${currentUser.id}`

      await kv.set(probeKey, {
        touchedAt: new Date().toISOString(),
      })

      const kvValue = await kv.get(probeKey)
      const document = await db.insert('_engine_probe', {
        userId: currentUser.id,
        tenantId: currentUser.tenantId,
        createdAt: new Date().toISOString(),
      })

      return json({
        ok: true,
        user: currentUser,
        kv: kvValue,
        db: document,
      })
    }

    if (url.pathname === '/__job' && request.method === 'POST') {
      const payload = (await request.json()) as {
        jobId?: string
        method?: string
        params?: unknown
      }
      const jobKey = `job:last:${payload.jobId ?? 'unknown'}`
      await kv.set(jobKey, {
        method: payload.method ?? '',
        params: payload.params ?? null,
        touchedAt: new Date().toISOString(),
      })
      return json({ ok: true, jobId: payload.jobId ?? null })
    }

    return json({ error: 'not_found' }, 404)
  },
})
