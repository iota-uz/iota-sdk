/**
 * Helpers of the «My tasks» browser scenarios: people of a role with passwords, a graph with a human
 * node started through the automations RPC, a signed-in session per person.
 */
import type { Browser, BrowserContext, Page } from '@playwright/test';
import bcrypt from 'bcryptjs';
import { randomUUID } from 'node:crypto';
import { expect } from '../../fixtures/test';
import { login } from '../../fixtures/auth';
import { withDatabase } from '../../fixtures/database';

export const PASSWORD = 'TestPass123!';
export const ADMIN = 'test@gmail.com';

export type Person = { id: number; email: string; name: string };

export async function people(names: string[]): Promise<{ role: number; members: Person[]; outsider: Person }> {
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
		// The role grants no permission at all: an assignee needs none to see and decide their tasks.
		for (const m of members) await db.query('INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)', [m.id, role.id]);
		return { role: Number(role.id), members, outsider };
	});
}

/** Saves, enables and starts a graph whose human node asks params.assignees; returns the run. */
export async function startTask(page: Page, title: string, params: Record<string, unknown>): Promise<string> {
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

export async function sessionOf(browser: Browser, baseURL: string | undefined, who: Person, viewport = { width: 1440, height: 900 }, options: Parameters<Browser['newContext']>[0] = {}): Promise<{ context: BrowserContext; page: Page }> {
	const context = await browser.newContext({ baseURL, viewport, ...options });
	const page = await context.newPage();
	await login(page, who.email, PASSWORD);
	return { context, page };
}

/** Opens the list until the task the engine is opening shows up. */
export async function rowOf(page: Page, title: string) {
	const row = page.getByTestId('automations-task-row').filter({ hasText: title });
	await expect(async () => {
		await page.goto('/automations/tasks');
		await expect(row).toHaveCount(1, { timeout: 1000 });
	}).toPass({ timeout: 60_000 });
	return row;
}

export async function decisions(task: string): Promise<number> {
	return withDatabase(async (db) => {
		const { rows: [r] } = await db.query('SELECT count(*)::int AS n FROM extruntime.human_task_decisions WHERE task_id = $1', [task]);
		return r.n as number;
	});
}


/**
 * The ID of the open task the run (startTask's result) opened, once the engine opened it. The run picks
 * the task, not its title: an open task of an earlier run with the same title is never returned.
 */
export async function taskIDOf(run: string): Promise<string> {
	let id = '';
	await expect.poll(async () => {
		id = await withDatabase(async (db) => {
			const { rows } = await db.query(
				`SELECT t.id FROM extruntime.human_tasks t
				 JOIN automation.runs r ON r.tenant_id = t.tenant_id AND r.instance_id = t.instance_id
				 WHERE r.id = $1 AND t.status = 'open'`,
				[run],
			);
			return (rows[0]?.id as string | undefined) ?? '';
		});
		return id;
	}, { timeout: 60_000 }).not.toBe('');
	return id;
}
