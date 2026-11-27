/**
 * «My tasks» screen states (#1348): every state of the list and of a task as screenshots at 1440 and
 * 390 px in the light and the dark theme (Q1, S1, V4) with axe on each (A1), a form the application
 * no longer has (H7), and touch: Chromium as a phone and WebKit as an iPad in both orientations (Q4).
 *
 * WebKit runs where Playwright's WebKit is installed (CI installs it); PLAYWRIGHT_WEBKIT_WS points the
 * scenario at a remote browser server instead (a host without WebKit's system libraries).
 */
import type { Browser, Page } from '@playwright/test';
import { devices, webkit } from '@playwright/test';
import { test, expect } from '../../fixtures/test';
import { login } from '../../fixtures/auth';
import { withDatabase } from '../../fixtures/database';
import { expectNoA11yViolations } from '../../../web/testing/src/a11y';
import { ADMIN, PASSWORD, type Person, people, rowOf, sessionOf, startTask, taskIDOf } from './inbox';

const FORM = {
	type: 'object',
	required: ['priority'],
	properties: {
		amount: { type: 'number', title: 'Approved amount' },
		priority: { type: 'integer', title: 'Priority', enum: [1, 2, 3] },
	},
};

/** Replaces the form of the pinned version's human node: the run asks for a form nobody registered. */
async function removeForm(title: string) {
	await withDatabase(async (db) => {
		await db.query(
			`UPDATE automation.versions v SET graph = jsonb_set(v.graph, '{nodes,0,params,form}', '{"type":"object","x-iota-form":"acme.removed"}')
			 FROM automation.automations a WHERE a.tenant_id = v.tenant_id AND a.id = v.automation_id AND a.name = $1`,
			[title],
		);
	});
}

type Shot = { name: string; open: (page: Page) => Promise<void> };

test.describe('automations: my tasks screen states', () => {
	test.setTimeout(420_000);

	test('states at 1440 and 390 px, light and dark, with axe (Q1, S1, A1, H7)', async ({ page, browser, baseURL }) => {
		const { role, members: [vera, otto] } = await people(['Vera', 'Otto']);
		await login(page, ADMIN, PASSWORD);
		const invoiceRun = await startTask(page, 'Approve the supplier invoice', {
			body: 'Invoice for **November** delivery.\n\n- Check the amount\n- Reject with a reason if it does not match',
			assignees: [{ kind: 'user', user_id: vera.id }], form: FORM, allow_delegate: true,
		});
		const actRun = await startTask(page, 'Sign the warehouse act', { assignees: [{ kind: 'user', user_id: vera.id }] });
		await removeForm('Sign the warehouse act');
		const vacationRun = await startTask(page, 'Confirm the vacation', { assignees: [{ kind: 'role', ref: String(role) }] });
		const invoice = await taskIDOf(invoiceRun);
		const act = await taskIDOf(actRun);
		const vacation = await taskIDOf(vacationRun);
		// Otto's own tasks: one Vera may not open, one that cannot be read (its stored escalation is damaged).
		const foreignRun = await startTask(page, 'Otto private approval', { assignees: [{ kind: 'user', user_id: otto.id }] });
		const damagedRun = await startTask(page, 'Otto damaged approval', { assignees: [{ kind: 'user', user_id: otto.id }] });
		const foreign = await taskIDOf(foreignRun);
		const damaged = await taskIDOf(damagedRun);
		await withDatabase(async (db) => {
			await db.query(`UPDATE extruntime.human_tasks SET escalation = '"damaged"'::jsonb WHERE id = $1`, [damaged]);
		});

		// H7: the form is gone: the task says so, takes no decision, and links the run for a reader.
		await page.goto(`/automations/tasks/${act}`);
		await expect(page.getByTestId('automations-task-missing')).toBeVisible(); // the admin is not an assignee
		const v = await sessionOf(browser, baseURL, vera);
		await v.page.goto(`/automations/tasks/${act}`);
		await expect(v.page.getByTestId('automations-task-form-unavailable')).toContainText('Form unavailable');
		await expect(v.page.getByTestId('automations-task-outcomes')).toHaveCount(0);
		// Vera cannot read runs: the panel that links the run for a reader has no link for her.
		await expect(v.page.getByTestId('automations-task-form-unavailable').getByRole('link', { name: 'Open the run' })).toHaveCount(0);
		await withDatabase(async (db) => {
			const { rows: [admin] } = await db.query('SELECT id FROM users WHERE email = $1', [ADMIN]);
			await db.query(`INSERT INTO extruntime.human_task_assignees (task_id, tenant_id, kind, user_id, ref, added_at)
				SELECT id, tenant_id, 'user', $2, '', clock_timestamp() FROM extruntime.human_tasks WHERE id = $1`, [act, admin.id]);
		});
		await page.goto(`/automations/tasks/${act}`);
		await expect(page.getByTestId('automations-task-form-unavailable').getByRole('link', { name: 'Open the run' })).toBeVisible();

		// Otto closes the vacation: Vera sees who decided it.
		const o = await sessionOf(browser, baseURL, otto);
		await o.page.goto(`/automations/tasks/${vacation}`);
		await o.page.getByTestId('automations-task-outcome-approve').click();
		await expect(o.page.getByText('Decision recorded').first()).toBeVisible();
		await o.context.close();
		// The moment of the decision is fixed, so the closed task's screenshot is the same in every run.
		await withDatabase(async (db) => {
			await db.query(`UPDATE extruntime.human_tasks SET decided_at = '2026-01-15T10:00:00Z' WHERE id = $1`, [vacation]);
			await db.query(`UPDATE extruntime.human_task_decisions SET decided_at = '2026-01-15T10:00:00Z' WHERE task_id = $1`, [vacation]);
		});
		await rowOf(v.page, 'Approve the supplier invoice');
		await v.context.close();

		const shots: Shot[] = [
			// S1/D3: a task of someone else, and a task that cannot be read; S4: a list that cannot be read.
			{ name: 'task-missing', open: async (p) => { await p.goto(`/automations/tasks/${foreign}`); } },
			{ name: 'task-failed', open: async (p) => {
				await p.goto(`/automations/tasks/${damaged}`);
				await expect(p.getByTestId('automations-task-missing').getByRole('alert')).toBeVisible();
			} },
			{ name: 'list-failed', open: async (p) => {
				await p.goto('/automations/tasks?cursor=unreadable');
				await expect(p.getByTestId('automations-tasks-failed')).toBeVisible();
			} },
			// Two tasks without a due time are ordered by their random IDs: the list is taken with a search
			// that leaves one, so every run shows the same rows.
			{ name: 'list', open: async (p) => { await p.goto('/automations/tasks?q=supplier'); } },
			{ name: 'list-nothing-found', open: async (p) => { await p.goto('/automations/tasks?q=zzzz'); } },
			{ name: 'list-empty', open: async (p) => { await p.goto('/automations/tasks?tab=decided'); } },
			{ name: 'task-form', open: async (p) => { await p.goto(`/automations/tasks/${invoice}`); } },
			{ name: 'task-confirm', open: async (p) => {
				await p.goto(`/automations/tasks/${invoice}`);
				await p.getByTestId('automations-task-outcome-reject').click();
				const confirm = p.getByTestId('automations-task-confirm-reject');
				await expect(confirm).toBeVisible();
				// The panel opens with an opacity and clip animation (iota-reveal). While it runs, Chromium
				// paints the panel on its own layer with grayscale text antialiasing, so a screenshot taken
				// then depends on timing. Wait for the panel at rest and one painted frame after it.
				await confirm.evaluate(async (el) => {
					await Promise.all(el.getAnimations().map((a) => a.finished));
					await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
				});
			} },
			{ name: 'task-form-unavailable', open: async (p) => { await p.goto(`/automations/tasks/${act}`); } },
			{ name: 'task-closed', open: async (p) => { await p.goto(`/automations/tasks/${vacation}`); } },
			{ name: 'task-offline', open: async (p) => {
				await p.goto(`/automations/tasks/${invoice}`);
				await p.context().setOffline(true);
				// The decision's send fails (htmx:sendError); the global toast handler runs on that same event,
				// so once it has fired and the next frame is painted, a toast would already be in the container.
				await p.evaluate(() => {
					const w = window as unknown as { decisionSendFailed?: boolean };
					w.decisionSendFailed = false;
					document.addEventListener('htmx:sendError', () => requestAnimationFrame(() => { w.decisionSendFailed = true; }), { once: true });
				});
				const failed = p.waitForEvent('requestfailed', (r) => r.method() === 'POST' && new URL(r.url()).pathname.endsWith('/decide'));
				await p.getByTestId('automations-task-outcome-approve').click();
				await failed;
				await p.waitForFunction(() => (window as unknown as { decisionSendFailed?: boolean }).decisionSendFailed === true);
				await expect(p.getByTestId('automations-tasks-offline')).toBeVisible();
				await expect(p.getByTestId('automations-tasks-offline')).toBeInViewport({ ratio: 1 });
				// The banner is the signal: no global toast over the decision buttons (F6).
				await expect(p.locator('[data-toast-container]').getByText('Check your connection and try again.')).toHaveCount(0);
				await p.context().setOffline(false);
			} },
		];
		for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
			for (const scheme of ['light', 'dark'] as const) {
				const s = await sessionOf(browser, baseURL, vera, viewport, { colorScheme: scheme });
				for (const shot of shots) {
					await shot.open(s.page);
					// No hover state under a pointer left on a button by the step: the screenshot and axe see
					// the screen at rest.
					await s.page.mouse.move(0, viewport.height - 1);
					await s.page.evaluate(() => document.fonts.ready);
					expect(await s.page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth),
						`${shot.name} ${viewport.width}: no horizontal scroll`).toBe(true);
					await expect(s.page).toHaveScreenshot(`${shot.name}-${viewport.width}-${scheme}.png`, {
						fullPage: true,
						// The bell's unread dot differs between runs.
						mask: [s.page.getByTestId('notification-bell')],
					});
					await expectNoA11yViolations(s.page, { include: ['#automations-tasks'] });
				}
				await s.context.close();
			}
		}
	});

	test('touch: a phone in Chromium and an iPad in WebKit decide with taps (T1, T6, Q4)', async ({ page, browser, baseURL }) => {
		const { members: [pia] } = await people(['Pia']);
		await login(page, ADMIN, PASSWORD);
		for (const title of ['Phone approval', 'iPad portrait approval', 'iPad landscape approval']) {
			await startTask(page, title, { assignees: [{ kind: 'user', user_id: pia.id }] });
		}

		// Chromium as a phone: real touch, the coarse pointer, 44 px targets, no horizontal scroll.
		const phone = await sessionOf(browser, baseURL, pia, devices['Pixel 7'].viewport, {
			...devices['Pixel 7'], viewport: devices['Pixel 7'].viewport,
		});
		await tapDecide(phone.page, 'Phone approval');
		await phone.context.close();

		// WebKit as an iPad, portrait and landscape.
		const ws = process.env.PLAYWRIGHT_WEBKIT_WS;
		// A remote browser server (another container) resolves a loopback app URL inside itself: forward the
		// loopback network of this host to it then. A non-loopback app URL must be reachable from there.
		const wk: Browser = ws ? await webkit.connect(ws, isLoopback(baseURL) ? { exposeNetwork: '<loopback>' } : {}) : await webkit.launch();
		try {
			for (const [title, device] of [['iPad portrait approval', devices['iPad Pro 11']], ['iPad landscape approval', devices['iPad Pro 11 landscape']]] as const) {
				const ipad = await sessionOf(wk, baseURL, pia, device.viewport, { ...device, viewport: device.viewport });
				await tapDecide(ipad.page, title);
				await ipad.context.close();
			}
		} finally {
			await wk.close();
		}
	});
});

/** Opens the task from the list by a tap and approves it by a tap; checks the touch targets. */
async function tapDecide(page: Page, title: string) {
	const row = await rowOf(page, title);
	await row.locator('a').first().tap();
	await expect(page.getByTestId('automations-task-screen')).toBeVisible();
	expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
	expect(await page.evaluate(() => matchMedia('(pointer: coarse)').matches)).toBe(true);
	const small = await page.evaluate(() => Array.from(document.querySelectorAll<HTMLElement>('#automations-tasks button, #automations-tasks a, #automations-tasks summary'))
		.filter((el) => el.offsetParent !== null)
		.map((el) => ({ el: el.textContent?.trim().slice(0, 30), h: el.getBoundingClientRect().height }))
		.filter((x) => x.h < 44));
	expect(small, 'every touch target is at least 44 px').toEqual([]);
	await page.getByTestId('automations-task-outcome-approve').tap();
	await expect(page.getByText('Decision recorded').first()).toBeVisible();
}

/** Whether the app URL points at this host's loopback (localhost, 127.0.0.0/8, ::1). */
function isLoopback(url: string | undefined): boolean {
	if (!url) return false;
	const host = new URL(url).hostname.replace(/^\[|\]$/g, '');
	return host === 'localhost' || host.endsWith('.localhost') || host === '::1' || /^127\./.test(host);
}
