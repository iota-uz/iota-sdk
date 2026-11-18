/**
 * Entry point for specs: the regular Playwright API with `test` bound to the
 * worker-owned environment (see environment.ts).
 */
export * from '@playwright/test';
export { test } from './environment';
