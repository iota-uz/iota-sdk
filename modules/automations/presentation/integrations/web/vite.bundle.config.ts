import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig, type Plugin } from 'vite'
import solid from 'vite-plugin-solid'

const web = path.dirname(fileURLToPath(import.meta.url))
const repo = path.resolve(web, '../../../../..')
const solidRoot = path.join(web, 'node_modules/solid-js')

// The SDK source commit of the client host is replaced by a sentinel that scripts/build-bundle.mjs turns
// into the content hash of the bundle: the bundle's identity, which the Go side sends back in the route
// context (dist/bundle.json), so the host's identity check passes without tying the bundle to a commit.
export const COMMIT_SENTINEL = '0123456789abcdef0123456789abcdef01234567'

const identity: Plugin = {
  name: 'automations-bundle-identity',
  enforce: 'pre',
  transform(code, id) {
    if (!id.split('?')[0]!.endsWith(path.join('client-host', 'src', 'identity.ts'))) return null
    return code.split('__IOTA_SDK_SOURCE_COMMIT__').join(COMMIT_SENTINEL)
  },
}

export default defineConfig({
  root: web,
  base: '/assets/automations/',
  plugins: [identity, solid()],
  resolve: {
    alias: [
      { find: '@iota-uz/sdk/solid-ui', replacement: path.join(repo, 'web/solid-ui/src/index.ts') },
      { find: '@iota-uz/sdk/solid', replacement: path.join(repo, 'web/client-host/src/solid-index.ts') },
      { find: 'solid-js/web', replacement: path.join(solidRoot, 'web/dist/web.js') },
      { find: 'solid-js/store', replacement: path.join(solidRoot, 'store/dist/store.js') },
      { find: /^solid-js$/, replacement: path.join(solidRoot, 'dist/solid.js') },
    ],
  },
  build: {
    outDir: path.resolve(web, '../dist'),
    emptyOutDir: true,
    manifest: true,
    sourcemap: false,
    target: 'es2022',
    rollupOptions: { input: { integrations: path.join(web, 'bundle/entry.ts') } },
  },
  logLevel: 'warn',
})
