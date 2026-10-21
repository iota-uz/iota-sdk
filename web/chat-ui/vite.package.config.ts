import { defineConfig } from 'vite';
import solid from 'vite-plugin-solid';
import path from 'node:path';
export default defineConfig({
 plugins: [solid()],
 build: { outDir: 'package-dist', emptyOutDir: true, cssCodeSplit: false,
  lib: { entry: { index: path.resolve('src/index.ts'), icons: path.resolve('src/icons/index.tsx') }, formats: ['es'] },
  rollupOptions: { external: ['solid-js', 'solid-js/web', 'solid-js/store', '@iota-uz/sdk/solid', '@iota-uz/sdk/client-host'], output: { entryFileNames: '[name].js', assetFileNames: 'styles.css' } }
 }
});
