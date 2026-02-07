import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

const sdkDist = path.resolve(__dirname, '../../../../dist')

export default defineConfig(({ command }) => ({
  plugins: [react()],
  resolve: {
    alias: [
      // In dev, resolve SDK to real dist/ (outside node_modules) so Vite
      // watches the files and triggers HMR when tsup --watch rebuilds them.
      ...(command === 'serve'
        ? [
            { find: /^@iota-uz\/sdk\/bichat$/, replacement: path.join(sdkDist, 'bichat/index.mjs') },
            { find: /^@iota-uz\/sdk$/, replacement: path.join(sdkDist, 'index.mjs') },
          ]
        : []),
      { find: '@', replacement: path.resolve(__dirname, './src') },
    ],
  },
  base: (() => {
    const base = process.env.APPLET_ASSETS_BASE || '/bi-chat/assets/'
    return base.endsWith('/') ? base : base + '/'
  })(),
  server: {
    port: Number(process.env.APPLET_VITE_PORT) || 5173,
    strictPort: true,
  },
  assetsInclude: ['**/*.css'],
  build: {
    outDir: '../assets/dist',
    emptyOutDir: true,
    manifest: true,
    cssCodeSplit: false, // Bundle all CSS into a single file
    rollupOptions: {
      output: {
        entryFileNames: 'assets/[name]-[hash].js',
        chunkFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash].[ext]',
      },
    },
  },
}))
