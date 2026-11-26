/**
 * «My tasks» (/automations/tasks): the acceptance of the screen in a browser, H1–H9 of #1348.
 *
 * Three people of one role decide a task with a quorum of 2 in their own sessions; a fourth takes a
 * delegated task from a notification. None of them holds any permission. The engine of the worker's
 * server runs the automations.
 */
import { randomUUID } from 'node:crypto';
import { test, expect } from '../../fixtures/test';
import { login } from '../../fixtures/auth';
import { expectNoA11yViolations } from '../../../web/testing/src/a11y';
import { ADMIN, PASSWORD, decisions, people, rowOf, sessionOf, startTask } from './inbox';

test.describe('automations: my tasks', () => {
	test.setTimeout(240_000);

	test('quorum of a role in three sessions, counter, confirmation, keyboard, offline and a11y (H1–H5, H8, S7, K7, A1)', async ({ page, browser, baseURL }) => {
		const { role, members: [anna, boris, vera] } = await people(['Anna', 'Boris', 'Vera']);
		await login(page, ADMIN, PASSWORD);
		const title = `Approve invoice ${randomUUID().slice(0, 6)}`;
		await startTask(page, title, { assignees: [{ kind: 'role', ref: String(role) }], required: 2 });

		const a = await sessionOf(browser, baseURL, anna);
		const b = await sessionOf(browser, baseURL, boris);
		const v = await sessionOf(browser, baseURL, vera, { width: 390, height: 844 });

		// H2: the row says where it came from and the quorum; the tabs are in the URL.
		const row = await rowOf(a.page, title);
		await expect(row.getByTestId('automations-task-via')).toHaveText(/Your role/);
		await expect(row.getByTestId('automations-task-quorum')).toHaveText(/0 of 2/);
		await a.page.getByTestId('automations-tasks-tab-overdue').click();
		await expect(a.page).toHaveURL(/tab=overdue/);
		await a.page.getByTestId('automations-tasks-tab-mine').click();
		await expectNoA11yViolations(a.page, { include: ['#automations-tasks'] });

		// H1: the navigation counter, a polite live region that is not recreated.
		const badge = a.page.getByTestId('automations-tasks-badge').first();
		await expect(badge).toHaveAttribute('aria-live', 'polite');
		await expect(badge).toContainText('1');

		// H3/H5: instructions as markdown; the danger outcome is confirmed above the buttons, Cancel first.
		await row.locator('a').first().click();
		const taskURL = a.page.url();
		const taskID = taskURL.split('/').pop()!.split('?')[0];
		await expect(a.page.getByTestId('automations-task-instructions').locator('strong')).toHaveText('amount');
		await a.page.getByTestId('automations-task-outcome-reject').click();
		const confirm = a.page.getByTestId('automations-task-confirm-reject');
		await expect(confirm).toBeVisible();
		await expect(confirm.getByRole('button', { name: 'Cancel' })).toBeFocused();
		await confirm.getByRole('button', { name: 'Cancel' }).click();
		await expect(confirm).toBeHidden();
		// A rejection without the comment it needs sends nothing and points at the comment.
		await a.page.getByTestId('automations-task-outcome-reject').click();
		await a.page.getByTestId('automations-task-confirm-submit-reject').click();
		await expect(a.page.getByTestId('automations-task-comment')).toBeFocused();
		expect(await decisions(taskID)).toBe(0);
		await expectNoA11yViolations(a.page, { include: ['#automations-tasks'] });

		// K7: ⌘/Ctrl+Enter in the comment presses the primary outcome; Enter alone breaks the line.
		const comment = a.page.getByTestId('automations-task-comment');
		await comment.fill('Matches the contract');
		await comment.press('Enter');
		await expect(comment).toHaveValue('Matches the contract\n');
		// H4: one decision; a toast; the counter refreshes in place (the live region is the same node).
		await badge.evaluate((el) => { (el as HTMLElement).dataset.marker = 'kept'; });
		await comment.press('Control+Enter');
		// The toast itself: the replaced list also says it in a screen-reader-only status.
		await expect(a.page.locator('[data-toast-container]').getByText('Decision recorded')).toBeVisible();
		expect(await decisions(taskID)).toBe(1);
		await expect(badge).not.toContainText('1');
		expect(await badge.evaluate((el) => (el as HTMLElement).dataset.marker)).toBe('kept');
		await a.page.goto('/automations/tasks?tab=decided');
		await expect(a.page.getByTestId('automations-task-row').filter({ hasText: title })).toContainText('You decided: Approve');

		// Vera keeps the task open on a phone while Boris closes the quorum.
		await rowOf(v.page, title);
		await v.page.goto(taskURL);
		const outcomes = v.page.getByTestId('automations-task-outcomes').getByRole('button');
		for (const button of await outcomes.all()) expect((await button.boundingBox())!.height).toBeGreaterThanOrEqual(44);
		expect(await v.page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
		await expectNoA11yViolations(v.page, { include: ['#automations-tasks'] });

		// S7: offline, the decision fails visibly; back online, Retry sends it once.
		await (await rowOf(b.page, title)).locator('a').first().click();
		await expect(b.page.getByTestId('automations-task-quorum')).toHaveText(/1 of 2/);
		await b.context.setOffline(true);
		await b.page.getByTestId('automations-task-outcome-approve').click();
		await expect(b.page.getByTestId('automations-tasks-offline')).toBeVisible();
		await b.context.setOffline(false);
		await b.page.getByTestId('automations-tasks-offline').getByRole('button', { name: 'Retry' }).click();
		await expect(b.page.getByText('Decision recorded').first()).toBeVisible();
		expect(await decisions(taskID)).toBe(2);

		// H4: the third session decides a task the others closed: who closed it, read only.
		await v.page.getByTestId('automations-task-outcome-approve').click();
		await expect(v.page.getByTestId('automations-task-closed')).toContainText('Already decided by');
		await expect(v.page.getByTestId('automations-task-outcomes')).toHaveCount(0);
		expect(await decisions(taskID)).toBe(2);

		for (const s of [a, b, v]) await s.context.close();
	});

	test('delegation from the notification link and a task the user cannot open (H6, H9, D3)', async ({ page, browser, baseURL }) => {
		const { members: [gleb], outsider: oleg } = await people(['Gleb']);
		await login(page, ADMIN, PASSWORD);
		const title = `Review contract ${randomUUID().slice(0, 6)}`;
		await startTask(page, title, { assignees: [{ kind: 'user', user_id: gleb.id }], allow_delegate: true });

		const g = await sessionOf(browser, baseURL, gleb);
		await (await rowOf(g.page, title)).locator('a').first().click();
		const taskURL = new URL(g.page.url()).pathname;
		await g.page.getByTestId('automations-task-delegate').locator('summary').click();
		const picker = g.page.getByRole('combobox', { name: 'Colleague' });
		await picker.fill(oleg.email);
		await expect(g.page.getByRole('option')).toHaveCount(1);
		await expect(g.page.getByRole('option').first()).toContainText(oleg.name);
		await picker.press('ArrowDown');
		await picker.press('Enter');
		await expect(picker).toHaveValue(oleg.name);
		await g.page.getByTestId('automations-task-delegate-submit').click();
		await expect(g.page.getByText(`Passed to ${oleg.name}`).first()).toBeVisible();
		await g.page.goto('/automations/tasks');
		await expect(g.page.getByTestId('automations-task-row').filter({ hasText: title })).toHaveCount(0);
		// D3: the task is no longer Gleb's: a clear screen, not an error page.
		await g.page.goto(taskURL);
		await expect(g.page.getByTestId('automations-task-missing')).toBeVisible();

		// H9: the delegate's notification opens the task at once, «Passed from Gleb».
		const o = await sessionOf(browser, baseURL, oleg);
		await expect(async () => {
			await o.page.goto('/notifications');
			await expect(o.page.getByTestId('notification').filter({ hasText: title })).toHaveCount(1, { timeout: 1000 });
		}).toPass({ timeout: 60_000 });
		await o.page.getByTestId('notification').filter({ hasText: title }).locator(`a[href$="${taskURL}"]`).first().click();
		await expect(o.page).toHaveURL(new RegExp(`${taskURL}$`));
		await expect(o.page.getByTestId('automations-task-via')).toContainText(`Passed from ${gleb.name}`);
		await expectNoA11yViolations(o.page, { include: ['#automations-tasks'] });

		for (const s of [g, o]) await s.context.close();
	});
});
