/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import solid from 'vite-plugin-solid'
import path from 'path'
import {
  createAppletViteConfig,
  createBichatStylesPlugin,
} from '@iota-uz/sdk/applet/vite'

export default defineConfig(({ command }) =>
  createAppletViteConfig({
    basePath: '/bi-chat',
    backendUrl: 'http://localhost:3900',
    enableLocalSdkAliases: command === 'serve',
    extend: {
      plugins: [
        solid(),
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
      test: {
        environment: 'jsdom',
        include: ['src/**/*.test.{ts,tsx}'],
      },
    },
  })
)
