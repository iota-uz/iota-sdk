import { defineConfig } from 'tsup';
import fs from 'fs';
import path from 'path';
import postcss from 'postcss';
import tailwindcss from 'tailwindcss';
import autoprefixer from 'autoprefixer';

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
  },
  async onSuccess() {
    // CSS processing
    const inputCSS = fs.readFileSync(path.resolve('./app/globals.css'), 'utf8');
    
    // Process with PostCSS and Tailwind
    const result = await postcss([
      tailwindcss({
        config: './tailwind.config.ts'
      }),
      autoprefixer()
    ]).process(inputCSS, {
      from: path.resolve('./app/globals.css'),
      to: path.resolve('./dist/styles.css')
    });
    
    // Write the processed CSS to the output file
    fs.writeFileSync(path.resolve('./dist/styles.css'), result.css);
    console.log('✅ Generated CSS bundle: dist/styles.css');
  }
});
