type RuntimeSDK = typeof import('@iota-uz/sdk/applet-runtime')

async function loadRuntimeSDK(): Promise<RuntimeSDK> {
  try {
    return await import('@iota-uz/sdk/applet-runtime')
  } catch {
    return await import('../../../../applets/ui/src/applet-runtime/index.ts')
  }
}

const { auth, db, defineApplet, kv } = await loadRuntimeSDK()

function json(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: {
      'content-type': 'application/json; charset=utf-8',
    },
  })
}

defineApplet({
  async fetch(request) {
    const url = new URL(request.url)

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
