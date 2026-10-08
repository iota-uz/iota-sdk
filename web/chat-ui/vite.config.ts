import { defineConfig } from 'vite';
import solid from 'vite-plugin-solid';
import tailwindcss from '@tailwindcss/postcss';
export default defineConfig({ plugins: [solid()], css:{postcss:{plugins:[tailwindcss()]}}, resolve: {dedupe:['solid-js']}, test: { environment: 'jsdom', include:['src/**/*.test.{ts,tsx}'] } });
