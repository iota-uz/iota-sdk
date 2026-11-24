import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import solid from 'vite-plugin-solid'

const web = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const repo = path.resolve(web, '../../../../..')
const solidRoot = path.join(web, 'node_modules/solid-js')

// The fixture stands in for a consumer application: the standard host, the UI kit and the editor come
// from the workspace sources, with a single solid-js instance.
export default defineConfig({
  plugins: [solid()],
  resolve: {
    alias: [
      { find: '@iota-uz/sdk/automation-editor/styles.css', replacement: path.join(repo, 'web/automation-editor/src/styles.css') },
      { find: '@iota-uz/sdk/automation-editor', replacement: path.join(repo, 'web/automation-editor/src/index.ts') },
      { find: '@xyflow/system', replacement: path.join(web, 'node_modules/@xyflow/system') },
      { find: '@iota-uz/sdk/solid-ui', replacement: path.join(repo, 'web/solid-ui/src/index.ts') },
      { find: '@iota-uz/sdk/solid', replacement: path.join(repo, 'web/client-host/src/solid-index.ts') },
      { find: 'solid-js/web', replacement: path.join(solidRoot, 'web/dist/web.js') },
      { find: 'solid-js/store', replacement: path.join(solidRoot, 'store/dist/store.js') },
      { find: /^solid-js$/, replacement: path.join(solidRoot, 'dist/solid.js') },
    ],
  },
  // /assets/fonts is where the application serves the SDK's fonts (iota.css refers to it).
  publicDir: path.join(repo, 'modules/core/presentation'),
  server: { fs: { allow: [repo] } },
})
