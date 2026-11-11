import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import solid from 'vite-plugin-solid'

const web = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const solidRoot = path.join(web, 'node_modules/solid-js')
const clientHost = path.resolve(web, '../../../../web/client-host/src')

// The fixture stands in for a consumer application: it resolves the standard
// host from the workspace source and keeps a single solid-js instance.
export default defineConfig({
  plugins: [solid()],
  resolve: {
    alias: [
      { find: '@iota-uz/sdk/solid', replacement: path.join(clientHost, 'solid-index.ts') },
      { find: 'solid-js/web', replacement: path.join(solidRoot, 'web/dist/web.js') },
      { find: 'solid-js/store', replacement: path.join(solidRoot, 'store/dist/store.js') },
      { find: /^solid-js$/, replacement: path.join(solidRoot, 'dist/solid.js') },
    ],
  },
  server: { fs: { allow: [path.resolve(web, '../../../..')] } },
})
