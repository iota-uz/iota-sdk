import { spawnSync } from 'node:child_process'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import solid from 'vite-plugin-solid'
import { defineConfig } from 'vite'

const rootDir = path.dirname(fileURLToPath(import.meta.url))

/** Story ids must match the code scan the VR manifest test compares against. */
function storyMeta() {
  return {
    name: 'lens-story-meta',
    closeBundle() {
      const script = path.resolve(rootDir, 'scripts/generate-story-meta.mjs')
      const result = spawnSync(process.execPath, [script, path.resolve(rootDir, 'dist')], { stdio: 'inherit' })
      if (result.status !== 0) throw new Error('story meta generation failed')
    },
  }
}

/** Serve the harness at `/` in dev (build output already maps `/` via the preview server). */
function storyRoot() {
  return {
    name: 'lens-story-root',
    configureServer(server: { middlewares: { use: (fn: (req: { url?: string }, res: unknown, next: () => void) => void) => void } }) {
      server.middlewares.use((req, _res, next) => {
        const url = (req.url ?? '').split('?')[0]
        if (url === '/') {
          req.url = `/stories.html${(req.url ?? '').slice(1)}`
        }
        next()
      })
    },
  }
}

export default defineConfig({
  appType: 'mpa',
  resolve: {
    alias: {
      '@iota-uz/sdk/client-host': path.resolve(rootDir, '../client-host/src/solid-index.ts'),
    },
  },
  plugins: [solid(), storyMeta(), storyRoot()],
  server: {
    port: 61000,
    watch: { usePolling: true, interval: 300 },
  },
  build: {
    outDir: path.resolve(rootDir, 'dist'),
    emptyOutDir: true,
    rollupOptions: {
      input: path.resolve(rootDir, 'stories.html'),
    },
  },
})
