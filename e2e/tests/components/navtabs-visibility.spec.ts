import { expect, test } from '@playwright/test';
import { readFile } from 'node:fs/promises';
import path from 'node:path';

test('workspace indicator follows first opening of a hidden sidebar', async ({ page }) => {
	const assets = path.resolve(__dirname, '../../../modules/core/presentation/assets');
	await page.route('http://sdk-fixture.test/**', async (route) => {
		const asset = new URL(route.request().url()).pathname;
		if (asset.startsWith('/assets/')) {
			await route.fulfill({ contentType: 'text/javascript', body: await readFile(path.join(assets, asset.slice('/assets/'.length))) });
			return;
		}
		await route.fulfill({ contentType: 'text/html', body: `
			<html><head><script type="module" src="/assets/js/alpine.js"></script></head><body>
			<div x-data="{isCollapsed: true}">
				<button @click="isCollapsed = false">Expand sidebar</button>
				<div x-show="!isCollapsed" x-transition>
					<div x-data="navTabs('erp')">
						<div x-ref="tabsContainer" style="display:flex;position:relative;width:260px">
							<div data-indicator :style="'width:'+backgroundStyle.width+'px;left:'+backgroundStyle.left+'px'"></div>
							<button data-tab-value="erp" style="width:130px">ERP</button>
							<button data-tab-value="edo" style="width:130px">EDO</button>
						</div>
					</div>
				</div>
			</div></body></html>` });
	});
	await page.goto('http://sdk-fixture.test/');
	await expect(page.locator('[data-tab-value="erp"]')).toBeHidden();
	await page.getByRole('button', { name: 'Expand sidebar' }).click();
	await expect(page.locator('[data-tab-value="erp"]')).toBeVisible();
	await expect.poll(() => page.locator('[data-indicator]').evaluate((element) => element.style.width)).toBe('130px');
});
