import { expect, test } from '@playwright/test';
import { readFile } from 'node:fs/promises';
import path from 'node:path';

// The scaffold table form's default hx-trigger is built by TableConfig.ResolvedHxTrigger:
// a shared guard constant joined into one clause per event. It is evaluated from the Go
// source so this test follows the production trigger instead of a copy of it.
async function defaultTableTrigger(root: string): Promise<string> {
	const source = await readFile(path.join(root, 'components/scaffold/table/mobile.go'), 'utf8');
	const guard = source.match(/const formTriggerGuard = "([^"\n]+)"/)?.[1];
	expect(guard, 'formTriggerGuard constant').toBeDefined();
	const body = source.match(/func \(c \*TableConfig\) ResolvedHxTrigger\(\) string \{[\s\S]*?strings\.Join\(\[\]string\{([\s\S]*?)\}, ", "\)/)?.[1];
	expect(body, 'ResolvedHxTrigger clause list').toBeDefined();
	const clauses = body!.split('\n').map((line) => line.trim().replace(/,$/, '')).filter(Boolean).map((line) => {
		expect(line, 'ResolvedHxTrigger clause').toMatch(/^("[^"]*"|g)( \+ ("[^"]*"|g))*$/);
		return line.split(' + ').map((part) => (part === 'g' ? guard! : part.slice(1, -1))).join('');
	});
	expect(clauses.length).toBeGreaterThan(1);
	return clauses.join(', ');
}

test('select changes submit current derived date fields on the first and subsequent requests', async ({ page }) => {
	const root = path.resolve(__dirname, '../../..');
	const assets = path.join(root, 'modules/core/presentation/assets');
	const trigger = await defaultTableTrigger(root);
	const yesterday = ['2026-10-23T19:00:00Z', '2026-10-24T19:00:00Z'];
	const today = ['2026-10-24T19:00:00Z', '2026-10-25T19:00:00Z'];
	await page.route('http://sdk-fixture.test/**', async (route) => {
		const url = new URL(route.request().url());
		if (url.pathname.startsWith('/assets/')) {
			await route.fulfill({ contentType: 'text/javascript', body: await readFile(path.join(assets, url.pathname.slice('/assets/'.length))) });
			return;
		}
		if (url.pathname === '/filter') {
			await route.fulfill({ contentType: 'text/html', body: '<p>Filtered</p>' });
			return;
		}
		await route.fulfill({ contentType: 'text/html', body: `
			<html><head>
				<script src="/assets/js/lib/htmx.min.js"></script>
				<script type="module" src="/assets/js/alpine.js"></script>
			</head><body>
				<form hx-get="/filter" hx-trigger="${trigger}" hx-target="#results" x-data="{createdAt:''}">
					<fieldset disabled hidden>
						<input type="hidden" name="CreatedAt.From" value="">
						<input type="hidden" name="CreatedAt.To" value="">
					</fieldset>
					<select aria-label="Period" x-model="createdAt">
						<option value="">All time</option>
						<option value="${yesterday.join('|')}">Yesterday</option>
						<option value="${today.join('|')}">Today</option>
					</select>
					<input type="hidden" name="CreatedAt.From" :value="createdAt.split('|')[0]">
					<input type="hidden" name="CreatedAt.To" :value="createdAt.split('|')[1] || ''">
					<div id="results"></div>
				</form>
			</body></html>` });
	});
	await page.goto('http://sdk-fixture.test/');
	for (const range of [yesterday, today, ['', '']]) {
		const request = page.waitForRequest((request) => new URL(request.url()).pathname === '/filter');
		await page.getByRole('combobox', { name: 'Period' }).selectOption(range[0] ? range.join('|') : '');
		const query = new URL((await request).url()).searchParams;
		await expect(page.locator('input[name="CreatedAt.From"]:enabled')).toHaveValue(range[0]);
		await expect(page.locator('input[name="CreatedAt.To"]:enabled')).toHaveValue(range[1]);
		expect(query.getAll('CreatedAt.From')).toEqual([range[0]]);
		expect(query.getAll('CreatedAt.To')).toEqual([range[1]]);
	}
});
