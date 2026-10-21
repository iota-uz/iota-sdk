/// <reference types="vitest/config" />
import {defineConfig} from 'vite';
import solid from 'vite-plugin-solid';
import path from 'node:path';
export default defineConfig({
 base:process.env.APPLET_ASSETS_BASE??'/bi-chat/assets/',
 plugins:[solid()],
 resolve:{dedupe:['solid-js'],alias:{'@':path.resolve(__dirname,'src')}},
 server:{host:'127.0.0.1',port:5173,proxy:{'/bi-chat/rpc':'http://localhost:3900','/bi-chat/stream':'http://localhost:3900'}},
 build:{outDir:'../assets/dist',emptyOutDir:true,manifest:true,cssCodeSplit:false,rollupOptions:{output:{entryFileNames:'assets/[name]-[hash].js',chunkFileNames:'assets/[name]-[hash].js',assetFileNames:'assets/[name]-[hash].[ext]'}}},
 test:{environment:'jsdom',include:['src/**/*.test.{ts,tsx}']},
});
