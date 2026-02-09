import { defineConfig } from 'tsup'

export default defineConfig({
  entry: {
    index: 'ui/src/index.ts',
    'bichat/index': 'ui/src/bichat/index.ts',
    'bichat/tailwind': 'ui/src/bichat/tailwind.ts',
    'applet/vite': 'ui/src/applet-vite/index.ts',
  },
  outDir: 'dist',
  format: ['esm', 'cjs'],
  outExtension({ format }) {
    return { js: format === 'esm' ? '.mjs' : '.cjs' }
  },
  dts: true,
  sourcemap: true,
  clean: true,
  treeshake: true,
  splitting: false,
  external: [
    'react',
    'react-dom',
    'node:fs',
    'node:path',
    'node:module',
    'node:child_process',
    'node:os',
  ],
})
