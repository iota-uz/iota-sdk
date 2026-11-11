import path from 'node:path'
import { fileURLToPath } from 'node:url'
// vite and vite-plugin-solid are installed with web/client-host; the browser fixture borrows them.
import { defineConfig } from '../../../../client-host/node_modules/vite/dist/node/index.js'
import solid from '../../../../client-host/node_modules/vite-plugin-solid/dist/esm/index.mjs'

const here = path.dirname(fileURLToPath(import.meta.url))
const clientHost = path.resolve(here, '../../../../client-host')
const repo = path.resolve(here, '../../../../..')

// Stands in for pkg/extruntime/ui: POST /_ext/rpc answers like the Go handler.
function rpc() {
  return {
    name: 'mock-ext-rpc',
    configureServer(server: { middlewares: { use(path: string, handler: (req: any, res: any) => void): void } }) {
      server.middlewares.use('/_ext/rpc', (req: any, res: any) => {
        const chunks: Buffer[] = []
        req.on('data', (c: Buffer) => chunks.push(c))
        req.on('end', () => {
          const body = JSON.parse(Buffer.concat(chunks).toString() || '{}') as { resource?: string }
          res.setHeader('content-type', 'application/json')
          if (body.resource === 'acme.insurance.expiring') {
            res.end(JSON.stringify({ ok: true, value: { count: 3 } }))
            return
          }
          res.statusCode = 422
          res.end(JSON.stringify({ ok: false, error: { kind: 'domain', code: 'acme.insurance.notActive', params: { status: 'draft' } } }))
        })
      })
    },
  }
}

export default defineConfig({
  root: here,
  plugins: [solid(), rpc()],
  resolve: {
    alias: {
      '@iota-uz/sdk/client-host': path.join(clientHost, 'src/solid-index.ts'),
    },
    dedupe: ['solid-js'],
  },
  server: { fs: { allow: [repo] } },
})
