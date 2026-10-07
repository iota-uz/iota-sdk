import { defineConfig } from 'astro/config';
import mdx from '@astrojs/mdx';
import solid from '@astrojs/solid-js';
import tailwind from '@tailwindcss/vite';
import { docsMarkdown } from './src/lib/markdown.mjs';
export default defineConfig({
 base:'/iota-sdk',output:'static',outDir:'./out',build:{format:'file'},
 integrations:[mdx(),solid()],
 markdown:{remarkPlugins:[[docsMarkdown,{base:'/iota-sdk'}]]},
 vite:{plugins:[tailwind()],resolve:{alias:{'@docs/builtins':new URL('./src/lib/builtins.tsx',import.meta.url).pathname}}},
});
