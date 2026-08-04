import path from 'node:path'
import { fileURLToPath } from 'node:url'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

const rootDir = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig({
  // Relative base, verified by experiment: removing it does NOT change the
  // dist bundle (Granite loads the runtime from manifest-derived absolute
  // URLs and never serves dist/index.html), but it DOES break the Ladle story
  // build, which shares this config and cannot resolve `/src/main.tsx` from
  // index.html without it — which takes the whole visual-regression lane with
  // it. The "Unable to preload CSS for /assets/index-<hash>.css" error that
  // first motivated this line was a stale dist pointing at a CSS chunk that
  // no longer existed, not an asset-path bug.
  base: './',
  plugins: [react()],
  server: {
    port: 5174,
    proxy: {
      '/lens': {
        target: process.env.LENS_BACKEND_URL ?? 'http://localhost:3200',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: path.resolve(rootDir, '../../pkg/lens/render/react/dist'),
    emptyOutDir: true,
    manifest: true,
    cssCodeSplit: true,
    rollupOptions: {
      input: path.resolve(rootDir, 'index.html'),
      output: {
        entryFileNames: 'assets/lens-dashboard-[hash].js',
        chunkFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash].[ext]',
      },
    },
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.{ts,tsx}'],
    setupFiles: './src/test/setup.ts',
    restoreMocks: true,
  },
})
