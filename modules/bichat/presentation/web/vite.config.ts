import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  base: process.env.APPLET_ASSETS_BASE || '/bi-chat/assets/',
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
})
