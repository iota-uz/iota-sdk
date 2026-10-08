import { defineConfig } from 'astro/config';
import mdx from '@astrojs/mdx';
import solid from '@astrojs/solid-js';
import tailwind from '@tailwindcss/vite';
import { docsMarkdown, docsTypography } from './src/lib/markdown.mjs';
export default defineConfig({
 devToolbar:{enabled:false},
 base:'/iota-sdk',output:'static',outDir:'./out',build:{format:'file'},
 integrations:[mdx(),solid()],
 markdown:{shikiConfig:{themes:{light:"github-light",dark:"github-dark"},defaultColor:false},rehypePlugins:[docsTypography],remarkPlugins:[[docsMarkdown,{base:'/iota-sdk'}]]},
 vite:{plugins:[tailwind()],resolve:{alias:{'@docs/builtins':new URL('./src/lib/builtins.tsx',import.meta.url).pathname}}},
});
