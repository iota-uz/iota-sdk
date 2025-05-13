import { defineConfig } from 'tsup';

export default defineConfig({
  entry: ['src/index.ts'],
  format: ['cjs', 'esm'],
  dts: true,
  tsconfig: './tsconfig.tsup.json',
  splitting: false,
  sourcemap: true,
  clean: true,
  treeshake: true,
  external: [
    'react',
    'react-dom',
    'next',
    'next-themes',
    '@radix-ui/*',
    'clsx',
    'tailwind-merge',
    'lucide-react'
  ],
  esbuildOptions(options) {
    options.resolveExtensions = ['.tsx', '.ts', '.jsx', '.js'];
    options.loader = {
      '.tsx': 'tsx',
      '.ts': 'tsx'
    };
  }
});
