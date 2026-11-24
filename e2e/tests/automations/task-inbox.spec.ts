/**
 * «My tasks» (/automations/tasks): the acceptance of the screen in a browser, H1–H9 of #1348.
 *
 * Three people of one role decide a task with a quorum of 2 in their own sessions; a fourth takes a
 * delegated task from a notification. The engine of the worker's server runs the automations.
 */
import type { Browser, BrowserContext, Page } from '@playwright/test';
import bcrypt from 'bcryptjs';
import { randomUUID } from 'node:crypto';
import { test, expect } from '../../fixtures/test';
import { login } from '../../fixtures/auth';
import { withDatabase } from '../../fixtures/database';
import { expectNoA11yViolations } from '../../../web/testing/src/a11y';

const PASSWORD = 'TestPass123!';
const ADMIN = 'test@gmail.com';

type Person = { id: number; email: string; name: string };

async function people(names: string[]): Promise<{ role: number; members: Person[]; outsider: Person }> {
	const hash = await bcrypt.hash(PASSWORD, 10);
	return withDatabase(async (db) => {
		const { rows: [admin] } = await db.query('SELECT tenant_id FROM users WHERE email = $1', [ADMIN]);
		const tag = randomUUID().slice(0, 8);
		const create = async (name: string): Promise<Person> => {
			const email = `${name.toLowerCase()}-${tag}@example.test`;
			const { rows: [u] } = await db.query(
				`INSERT INTO users (first_name, last_name, email, password, ui_language, type, tenant_id) VALUES ($1, 'Inbox', $2, $3, 'en', 'user', $4) RETURNING id`,
				[name, email, hash, admin.tenant_id],
			);
			return { id: Number(u.id), email, name: `${name} Inbox` };
		};
		const members: Person[] = [];
		for (const name of names) members.push(await create(name));
		const outsider = await create('Delegate');
		const { rows: [role] } = await db.query(`INSERT INTO roles (type, name, tenant_id) VALUES ('user', $1, $2) RETURNING id`, [`Approvers ${tag}`, admin.tenant_id]);
		// A user without any permission gets no navigation at all: the role grants one unrelated to
		// automations, so the counter is checked as people see it.
		const { rows: [perm] } = await db.query(`SELECT id FROM permissions WHERE name NOT LIKE 'Automations.%' AND name NOT LIKE 'Connections.%' ORDER BY name LIMIT 1`);
		await db.query('INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)', [role.id, perm.id]);
		for (const m of members) await db.query('INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)', [m.id, role.id]);
		return { role: Number(role.id), members, outsider };
	});
}

/** Saves, enables and starts a graph whose human node asks params.assignees; returns the run. */
async function startTask(page: Page, title: string, params: Record<string, unknown>): Promise<string> {
	return page.evaluate(async ({ title, params }) => {
		const rpc = async (method: string, p: unknown) => {
			const r = await fetch('/automations/rpc', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ jsonrpc: '2.0', id: 1, method, params: p }) });
			const body = await r.json();
			if (body.error) throw new Error(`${method}: ${JSON.stringify(body.error)}`);
			return body.result;
		};
		const graph = {
			name: title, run_as: 'owner',
			triggers: [{ id: 'start', event: 'automation.manual.v1' }],
			nodes: [
				{ id: 'ask', type: 'human', params: Object.assign({
					title, body: 'Check the **amount** first.',
					buttons: [
						{ outcome: 'approve', label: 'Approve', style: 'primary' },
						{ outcome: 'reject', label: 'Reject', style: 'danger', require_comment: true },
					],
				}, params) },
				{ id: 'fin', type: 'end', params: {} },
			],
			edges: [
				{ from: { node: 'start', outcome: 'fired' }, to: 'ask' },
				{ from: { node: 'ask', outcome: 'approve' }, to: 'fin' },
				{ from: { node: 'ask', outcome: 'reject' }, to: 'fin' },
			],
		};
		const created = await rpc('automations.create', { graph, layout: {} });
		if (!created.ok) throw new Error(JSON.stringify(created));
		const review = await rpc('automations.enable_review', { id: created.id, version: created.version });
		const enabled = await rpc('automations.enable', { id: created.id, version: created.version, code_digest: review.code_digest });
		if (!enabled.ok) throw new Error(JSON.stringify(enabled));
		const started = await rpc('automations.run_manual', { id: created.id, trigger_id: 'start', input: {} });
		if (!started.ok) throw new Error(JSON.stringify(started));
		return started.run_id as string;
	}, { title, params });
}

async function sessionOf(browser: Browser, baseURL: string | undefined, who: Person, viewport = { width: 1440, height: 900 }): Promise<{ context: BrowserContext; page: Page }> {
	const context = await browser.newContext({ baseURL, viewport });
	const page = await context.newPage();
	await login(page, who.email, PASSWORD);
	return { context, page };
}

/** Opens the list until the task the engine is opening shows up. */
async function rowOf(page: Page, title: string) {
	const row = page.getByTestId('automations-task-row').filter({ hasText: title });
	await expect(async () => {
		await page.goto('/automations/tasks');
		await expect(row).toHaveCount(1, { timeout: 1000 });
	}).toPass({ timeout: 60_000 });
	return row;
}

async function decisions(task: string): Promise<number> {
	return withDatabase(async (db) => {
		const { rows: [r] } = await db.query('SELECT count(*)::int AS n FROM extruntime.human_task_decisions WHERE task_id = $1', [task]);
		return r.n as number;
	});
}

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
		await expect(a.page.getByText('Decision recorded').first()).toBeVisible();
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
