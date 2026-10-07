import { defineConfig } from 'vite';
import solid from 'vite-plugin-solid';
export default defineConfig({ plugins: [solid()], resolve: {dedupe:['solid-js']}, test: { environment: 'jsdom', include:['src/**/*.test.{ts,tsx}'] } });
