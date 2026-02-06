import { defineConfig } from 'tsup'

export default defineConfig({
  entry: {
    index: 'ui/src/index.ts',
    'bichat/index': 'ui/src/bichat/index.ts',
  },
  outDir: 'dist',
  format: ['esm'],
  outExtension() {
    return { js: '.mjs' }
  },
  dts: false,
  sourcemap: false,
  clean: true,
  treeshake: false,
  splitting: false,
  external: ['react', 'react-dom'],
})
