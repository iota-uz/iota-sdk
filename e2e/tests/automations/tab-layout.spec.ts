/**
 * Switching the automation editor between Edit and Runs moves nothing on the page: the app nav, the header
 * avatar, the graph's title and the tabs keep their boxes (within 1 px), and nothing scrolls sideways.
 */
import { test, expect, type Page } from '../../fixtures/test';
import { login } from '../../fixtures/auth';
import { ADMIN, PASSWORD } from './inbox';

const boxes = (page: Page) =>
	page.evaluate(() => {
		const box = (el: Element | null | undefined) => {
			if (!el) return null;
			const r = el.getBoundingClientRect();
			return [r.x, r.y, r.width, r.height];
		};
		const header = document.querySelector('section.h-16');
		const content = document.querySelector('.content');
		return {
			// The app nav column: the first column of the shell.
			nav: box(document.querySelector('.min-h-screen > div:first-child')),
			avatar: box(header?.lastElementChild),
			title: box(document.querySelector('.ae-toolbar__name')),
			tabs: box(document.querySelector('.ae-toolbar__tabs')),
			sideways: content ? content.scrollWidth - content.clientWidth : 0,
		};
	});

test.describe('automations: Edit and Runs share one layout', () => {
	for (const width of [1280, 1440, 1920]) {
		test(`nav, avatar, title and tabs stand still between the tabs (${width} px)`, async ({ page }) => {
			await page.setViewportSize({ width, height: 900 });
			await login(page, ADMIN, PASSWORD);
			await page.goto('/automations/new');
			await expect(page.locator('.ae-canvas')).toBeVisible();
			const edit = await boxes(page);
			await page.getByTestId('tab-runs').click();
			await expect(page.locator('.ae-runs-tab')).toBeVisible();
			const runs = await boxes(page);
			for (const key of ['nav', 'avatar', 'title', 'tabs'] as const) {
				const a = edit[key];
				const b = runs[key];
				expect(a, key).not.toBeNull();
				a!.forEach((v, i) => expect(Math.abs(v - b![i]!), `${key}: ${a} vs ${b}`).toBeLessThanOrEqual(1));
			}
			expect(edit.sideways).toBe(0);
			expect(runs.sideways).toBe(0);
			await expect(page.locator('.ae-save')).toBeHidden();
		});
	}
});
