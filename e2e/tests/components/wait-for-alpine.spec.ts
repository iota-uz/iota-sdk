import { expect, test } from '../../fixtures/test';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { waitForAlpine } from '../../fixtures/auth';

const assets = path.resolve(__dirname, '../../../modules/core/presentation/assets');

async function serve(page: import('@playwright/test').Page, head: string, body: string) {
	await page.route('http://sdk-fixture.test/**', async (route) => {
		const asset = new URL(route.request().url()).pathname;
		if (asset.startsWith('/assets/')) {
			await route.fulfill({ contentType: 'text/javascript', body: await readFile(path.join(assets, asset.slice('/assets/'.length))) });
			return;
		}
		await route.fulfill({ contentType: 'text/html', body: `<html><head>${head}</head><body>${body}</body></html>` });
	});
	await page.goto('http://sdk-fixture.test/');
}

// Regression: waitForAlpine swallowed the timeout and slept a fixed second, so a page without
// Alpine passed. Falsely green if the readiness predicate accepted a missing Alpine global.
test('rejects when Alpine never initializes', async ({ page }) => {
	await serve(page, '', '<div x-data="{ open: false }">Static</div>');
	await expect(waitForAlpine(page, 300)).rejects.toThrow();
});

// Regression: the old predicate accepted any Alpine global. Falsely green if readiness ignored
// whether Alpine initialized the component roots.
test('rejects while Alpine is loaded but components are not initialized', async ({ page }) => {
	await serve(page, '<script>window.Alpine = { version: "3.14.1" }</script>', '<div x-data="{ open: false }">Pending</div>');
	await expect(waitForAlpine(page, 300)).rejects.toThrow();
});

// Characterisation: falsely green if a fixed delay were reintroduced after readiness.
test('returns promptly once production Alpine initialized every component', async ({ page }) => {
	await serve(page, '<script type="module" src="/assets/js/alpine.js"></script>', '<div x-data="{ label: \'ready\' }"><span x-text="label"></span></div>');
	await expect(page.locator('span')).toHaveText('ready');
	const start = Date.now();
	await waitForAlpine(page);
	expect(Date.now() - start).toBeLessThan(500);
});
