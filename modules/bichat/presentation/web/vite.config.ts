import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'
import {
  createAppletViteConfig,
  createBichatStylesPlugin,
} from '@iota-uz/sdk/applet/vite'

const sdkDist = path.resolve(__dirname, '../../../../dist')

export default defineConfig(({ command }) =>
  createAppletViteConfig({
    basePath: '/bi-chat',
    backendUrl: 'http://localhost:3900',
    enableLocalSdkAliases: command === 'serve',
    sdkDistDir: command === 'serve' ? sdkDist : undefined,
    extend: {
      plugins: [
        react(),
        createBichatStylesPlugin({
          tailwindConfigPath: 'tailwind.config.js',
        }),
      ],
      resolve: {
        alias: [{ find: '@', replacement: path.resolve(__dirname, './src') }],
      },
      assetsInclude: ['**/*.css'],
      build: {
        outDir: '../assets/dist',
        emptyOutDir: true,
        manifest: true,
        cssCodeSplit: false,
        rollupOptions: {
          output: {
            entryFileNames: 'assets/[name]-[hash].js',
            chunkFileNames: 'assets/[name]-[hash].js',
            assetFileNames: 'assets/[name]-[hash].[ext]',
          },
        },
      },
    },
  })
)
