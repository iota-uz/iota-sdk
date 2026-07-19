import path from 'node:path'
import { fileURLToPath } from 'node:url'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

const rootDir = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig({
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
    cssCodeSplit: false,
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
    setupFiles: './src/test/setup.ts',
    restoreMocks: true,
  },
})
