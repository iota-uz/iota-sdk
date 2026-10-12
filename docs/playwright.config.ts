import {defineConfig} from '@playwright/test';
const port=Number(process.env.DOCS_TEST_PORT??45171);
export default defineConfig({testDir:'./tests',timeout:30000,workers:1,use:{baseURL:`http://127.0.0.1:${port}`,browserName:'chromium'},webServer:{command:`pnpm exec astro preview --host 127.0.0.1 --port ${port}`,url:`http://127.0.0.1:${port}/iota-sdk/`,reuseExistingServer:false,timeout:30000}});
