import {defineConfig} from '@playwright/test';
export default defineConfig({testDir:'./browser-tests',workers:1,use:{baseURL:'http://127.0.0.1:45173',browserName:'chromium'},webServer:{command:'pnpm vite fixtures/chat --config vite.config.ts --host 127.0.0.1 --port 45173 --strictPort',url:'http://127.0.0.1:45173',reuseExistingServer:false}});
