import path from 'node:path'
import { fileURLToPath } from 'node:url'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

const rootDir = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig({
  resolve: {
    alias: {
      '@iota-uz/sdk/client-host': path.resolve(rootDir, '../client-host/src/index.ts'),
    },
    dedupe: ['react', 'react-dom'],
  },
  plugins: [react()],
  build: {
    outDir: path.resolve(rootDir, 'package-dist'),
    emptyOutDir: true,
    cssCodeSplit: false,
    lib: { entry: path.resolve(rootDir, 'src/index.ts'), formats: ['es'], fileName: 'index' },
    rollupOptions: {
      external: ['react', 'react/jsx-runtime', 'react-dom', 'react-dom/client', '@iota-uz/sdk/client-host'],
      output: { assetFileNames: (asset) => asset.name?.endsWith('.css') ? 'style.css' : 'assets/[name]-[hash][extname]' },
    },
  },
})
