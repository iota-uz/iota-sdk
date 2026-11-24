/**
 * «My tasks» under failures (#1348): an API credential is refused with HUMAN_REQUIRED, a server that
 * answers 503 keeps what was typed and Retry repeats the action once (S4, S7), a late answer of the
 * colleague search never chooses for the person (K5).
 */
import { randomUUID } from 'node:crypto';
import { request as playwrightRequest } from '@playwright/test';
import { test, expect } from '../../fixtures/test';
import { login } from '../../fixtures/auth';
import { ADMIN, PASSWORD, decisions, people, rowOf, sessionOf, startTask, taskIDOf } from './inbox';

test.describe('automations: my tasks under failures', () => {
	test.setTimeout(240_000);

	test('an MCP bearer credential without a browser session gets HUMAN_REQUIRED', async ({ page, baseURL }) => {
		const { members: [gleb] } = await people(['Gleb']);
		await login(page, ADMIN, PASSWORD);
		const title = `Bearer ${randomUUID().slice(0, 6)}`;
		await startTask(page, title, { assignees: [{ kind: 'user', user_id: gleb.id }] });
		const task = await taskIDOf(title);

		const api = await playwrightRequest.newContext({ baseURL }); // no cookies
		for (const verb of ['decide', 'delegate']) {
			const r = await api.post(`/automations/tasks/${task}/${verb}`, {
				headers: { Authorization: 'Bearer mcp_pat_not_a_session' }, form: { outcome: 'approve', to: '1' }, maxRedirects: 0,
			});
			expect(r.status()).toBe(403);
			expect(r.headers()['x-error-code']).toBe('HUMAN_REQUIRED');
			expect((await r.json()).error.code).toBe('HUMAN_REQUIRED');
		}
		// Without any credential it is the login page, as for any page.
		const anonymous = await api.post(`/automations/tasks/${task}/decide`, { form: { outcome: 'approve' }, maxRedirects: 0 });
		expect([302, 303, 401]).toContain(anonymous.status());
		await api.dispose();
		expect(await decisions(task)).toBe(0);
	});

	test('503 keeps the comment and the colleague; Retry sends once, also from the keyboard (S4, S7)', async ({ page, browser, baseURL }) => {
		const { members: [anna, boris], outsider } = await people(['Anna', 'Boris']);
		await login(page, ADMIN, PASSWORD);
		const decideTitle = `Unavailable decide ${randomUUID().slice(0, 6)}`;
		const delegateTitle = `Unavailable delegate ${randomUUID().slice(0, 6)}`;
		await startTask(page, decideTitle, { assignees: [{ kind: 'user', user_id: anna.id }] });
		await startTask(page, delegateTitle, { assignees: [{ kind: 'user', user_id: boris.id }], allow_delegate: true });

		// A decision answered 503: the banner, the comment stays; Retry records it once.
		const a = await sessionOf(browser, baseURL, anna);
		await (await rowOf(a.page, decideTitle)).locator('a').first().click();
		const taskID = new URL(a.page.url()).pathname.split('/').pop()!;
		let unavailable = true;
		await a.page.route('**/decide*', async (route) => {
			if (unavailable) {
				unavailable = false;
				await route.fulfill({ status: 503, body: 'Service Unavailable' });
				return;
			}
			await route.continue();
		});
		await a.page.getByTestId('automations-task-comment').fill('Checked twice');
		await a.page.getByTestId('automations-task-outcome-approve').click();
		await expect(a.page.getByTestId('automations-tasks-offline')).toBeVisible();
		await expect(a.page.getByTestId('automations-task-comment')).toHaveValue('Checked twice');
		expect(await decisions(taskID)).toBe(0);
		await a.page.getByTestId('automations-tasks-offline').getByRole('button', { name: 'Retry' }).click();
		await expect(a.page.getByText('Decision recorded').first()).toBeVisible();
		expect(await decisions(taskID)).toBe(1);

		// A delegation sent with ⌘/Ctrl+Enter answered 503: the colleague and the comment stay; Retry
		// passes the task.
		const b = await sessionOf(browser, baseURL, boris);
		await (await rowOf(b.page, delegateTitle)).locator('a').first().click();
		let down = true;
		await b.page.route('**/delegate*', async (route) => {
			if (down && route.request().method() === 'POST') {
				down = false;
				await route.fulfill({ status: 503, body: 'Service Unavailable' });
				return;
			}
			await route.continue();
		});
		await b.page.getByTestId('automations-task-delegate').locator('summary').click();
		const picker = b.page.getByRole('combobox', { name: 'Colleague' });
		await picker.fill(outsider.email);
		await expect(b.page.getByRole('option')).toHaveCount(1);
		await picker.press('ArrowDown');
		await picker.press('Enter');
		const comment = b.page.getByLabel('Comment for the colleague');
		await comment.fill('Please take it');
		await comment.press('Control+Enter');
		await expect(b.page.getByTestId('automations-tasks-offline')).toBeVisible();
		await expect(comment).toHaveValue('Please take it');
		await expect(picker).toHaveValue(outsider.name);
		await b.page.getByTestId('automations-tasks-offline').getByRole('button', { name: 'Retry' }).click();
		await expect(b.page.getByText(`Passed to ${outsider.name}`).first()).toBeVisible();

		for (const s of [a, b]) await s.context.close();
	});

	test('a late answer of the colleague search never chooses: Enter waits for the current query (K5)', async ({ page, browser, baseURL }) => {
		const { members: [gleb], outsider } = await people(['Gleb']);
		await login(page, ADMIN, PASSWORD);
		const title = `Picker race ${randomUUID().slice(0, 6)}`;
		await startTask(page, title, { assignees: [{ kind: 'user', user_id: gleb.id }], allow_delegate: true });
		const g = await sessionOf(browser, baseURL, gleb);
		await (await rowOf(g.page, title)).locator('a').first().click();
		await g.page.getByTestId('automations-task-delegate').locator('summary').click();
		const picker = g.page.getByRole('combobox', { name: 'Colleague' });
		// The answer for "Gleb" (anyone but the delegate) comes late.
		await g.page.route('**/delegates?*', async (route) => {
			const q = new URL(route.request().url()).searchParams.get('q') ?? '';
			if (q === 'admin') await new Promise((r) => setTimeout(r, 1500));
			await route.continue();
		});
		await picker.fill('admin');
		await g.page.waitForTimeout(400); // the request for "admin" is on its way, slowly
		await picker.fill(outsider.email);
		await picker.press('ArrowDown');
		await picker.press('Enter');
		await expect(g.page.getByTestId('automations-task-delegate-submit')).toBeDisabled();
		await expect(g.page.getByRole('option')).toHaveCount(1);
		await expect(g.page.getByRole('option').first()).toContainText(outsider.name);
		await g.page.waitForTimeout(1600); // the late answer for "admin" arrives and is dropped
		await expect(g.page.getByRole('option')).toHaveCount(1);
		await picker.press('ArrowDown');
		await picker.press('Enter');
		await expect(picker).toHaveValue(outsider.name);
		await expect(g.page.getByTestId('automations-task-delegate-submit')).toBeEnabled();
		await g.context.close();
	});
});
