import { defineConfig } from '@playwright/test';

export default defineConfig({
	testDir: './tests/components',
	use: { browserName: 'chromium', viewport: { width: 390, height: 844 } },
});
