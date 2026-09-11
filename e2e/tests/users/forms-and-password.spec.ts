import { test, expect } from '@playwright/test';
import { login } from '../../fixtures/auth';
import { resetTestDatabase, seedScenario } from '../../fixtures/test-data';
import { withDatabase } from '../../fixtures/database';

test.describe('user form scalability and self-service password change', () => {
	test.describe.configure({ mode: 'serial' });

	test.beforeAll(async ({ request }) => {
		await resetTestDatabase(request, { reseedMinimal: false });
		await seedScenario(request, 'comprehensive');
	});

	test('create and edit forms expose every role and retain an assigned role after grant rights change', async ({ page }) => {
		// Falsely green if the fixture stays within the 25-row default page or the retained role is grantable.
		const fixture = await withDatabase(async (db) => {
			const tenant = await db.query(`SELECT tenant_id, id FROM users WHERE email = 'test@gmail.com' LIMIT 1`);
			const tenantID = tenant.rows[0].tenant_id as string;
			const adminID = tenant.rows[0].id as number;
			const target = await db.query(`SELECT id FROM users WHERE tenant_id = $1 AND id <> $2 ORDER BY id LIMIT 1`, [tenantID, adminID]);
			const roles = await db.query(
				`INSERT INTO roles (type, tenant_id, name, description, created_at, updated_at)
				 SELECT 'user', $1, 'Form role ' || LPAD(n::text, 2, '0'), '', NOW(), NOW()
				 FROM generate_series(1, 40) n RETURNING id`,
				[tenantID],
			);
			const deniedPermissionID = 'd03121ab-1556-4c2b-93b1-141f77bcb845';
			await db.query(
				`INSERT INTO permissions (id, name, resource, action, modifier, description)
				 VALUES ($1, 'form-retained-role-only', 'form-retained-secret', 'read', 'all', '')`,
				[deniedPermissionID],
			);
			const retainedRole = await db.query(
				`INSERT INTO roles (type, tenant_id, name, description, created_at, updated_at)
				 VALUES ('user', $1, 'Previously assigned role', '', NOW(), NOW()) RETURNING id`,
				[tenantID],
			);
			await db.query(`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`, [retainedRole.rows[0].id, deniedPermissionID]);
			await db.query(`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, [target.rows[0].id, retainedRole.rows[0].id]);
			return {
				targetID: target.rows[0].id as number,
				roleIDs: roles.rows.map((row) => String(row.id)),
				retainedRoleID: String(retainedRole.rows[0].id),
			};
		});

		await login(page, 'test@gmail.com', 'TestPass123!');
		for (const path of ['/users/new', `/users/${fixture.targetID}/edit`]) {
			await page.goto(path);
			for (const roleID of fixture.roleIDs) {
				await expect(page.locator(`select[name="RoleIDs"] option[value="${roleID}"]`)).toHaveCount(1);
			}
		}
		await expect(page.locator(`select[name="RoleIDs"] option[value="${fixture.retainedRoleID}"]`)).toBeSelected();
		await page.locator('select[name="RoleIDs"]').selectOption([fixture.retainedRoleID, fixture.roleIDs[0]]);
		await page.locator('[name="FirstName"]').fill('');
		await page.locator('#save-btn').click();
		await expect(page.locator(`select[name="RoleIDs"] option[value="${fixture.retainedRoleID}"]`)).toBeSelected();
		await expect(page.locator(`select[name="RoleIDs"] option[value="${fixture.roleIDs[0]}"]`)).toBeSelected();
	});

	test('create and edit forms stay responsive with a 20,000-member group', async ({ page }) => {
		// Falsely green if the large group is not grantable/rendered or if only one of the two routes is timed.
		const fixture = await withDatabase(async (db) => {
			const tenant = await db.query(`SELECT tenant_id, id FROM users WHERE email = 'test@gmail.com' LIMIT 1`);
			const tenantID = tenant.rows[0].tenant_id as string;
			const adminID = tenant.rows[0].id as number;
			const group = await db.query(
				`INSERT INTO user_groups (type, tenant_id, name, description, created_at, updated_at)
				 VALUES ('user', $1, 'Twenty thousand members', 'performance regression fixture', NOW(), NOW()) RETURNING id`,
				[tenantID],
			);
			await db.query(
				`INSERT INTO users (type, tenant_id, first_name, last_name, email, password, ui_language, created_at, updated_at)
				 SELECT 'user', $1, 'Bulk', n::text, 'bulk-member-' || n || '@example.test', '', 'en', NOW(), NOW()
				 FROM generate_series(1, 20000) n`,
				[tenantID],
			);
			await db.query(
				`INSERT INTO group_users (group_id, user_id)
				 SELECT $1, id FROM users WHERE tenant_id = $2 AND email LIKE 'bulk-member-%@example.test'`,
				[group.rows[0].id, tenantID],
			);
			const target = await db.query(`SELECT id FROM users WHERE tenant_id = $1 AND id <> $2 ORDER BY id LIMIT 1`, [tenantID, adminID]);
			return { groupID: group.rows[0].id as string, targetID: target.rows[0].id as number };
		});

		await login(page, 'test@gmail.com', 'TestPass123!');
		for (const path of ['/users/new', `/users/${fixture.targetID}/edit`]) {
			const started = Date.now();
			await page.goto(path, { timeout: 3000 });
			expect(Date.now() - started).toBeLessThan(3000);
			await expect(page.locator(`select[name="GroupIDs"] option[value="${fixture.groupID}"]`)).toHaveText('Twenty thousand members');
		}
		await page.locator('[name="LastName"]').fill('Preserved edit');
		await page.locator('select[name="GroupIDs"]').selectOption(fixture.groupID);
		await page.locator('[name="FirstName"]').fill('');
		await page.locator('#save-btn').click();
		await expect(page.locator('[name="LastName"]')).toHaveValue('Preserved edit');
		await expect(page.locator('select[name="GroupIDs"]')).toHaveValue(fixture.groupID);

		await page.goto('/users/new');
		await page.locator('[name="FirstName"]').fill('Preserved');
		await page.locator('select[name="GroupIDs"]').selectOption(fixture.groupID);
		await page.locator('#save-btn').click();
		await expect(page.locator('[name="FirstName"]')).toHaveValue('Preserved');
		await expect(page.locator('select[name="GroupIDs"]')).toHaveValue(fixture.groupID);
	});

	test('profile password form validates, revokes every session, and preserves password on profile edits', async ({ page, browser }) => {
		// Falsely green if only the current browser is logged out or login is not retried with both credentials.
		const email = 'test@gmail.com';
		const oldPassword = 'TestPass123!';
		const newPassword = 'ChangedPass123!';
		const otherContext = await browser.newContext();
		const otherPage = await otherContext.newPage();
		try {
			await login(page, email, oldPassword);
			await login(otherPage, email, oldPassword);
			await page.goto('/account');
			await expect(page.getByTestId('change-password-form')).toBeVisible();

			await page.getByTestId('current-password').fill(oldPassword);
			await page.getByTestId('new-password').fill(newPassword);
			await page.getByTestId('confirm-password').fill('DifferentPass123!');
			await page.getByTestId('change-password-submit').click();
			await expect(page.getByText('The password confirmation does not match.')).toBeVisible();

			await page.getByTestId('current-password').fill('WrongPass123!');
			await page.getByTestId('new-password').fill(newPassword);
			await page.getByTestId('confirm-password').fill(newPassword);
			await page.getByTestId('change-password-submit').click();
			await expect(page.getByText('The current password is incorrect.')).toBeVisible();

			await page.getByTestId('current-password').fill(oldPassword);
			await page.getByTestId('new-password').fill(newPassword);
			await page.getByTestId('confirm-password').fill(newPassword);
			await Promise.all([
				page.waitForURL((url) => url.pathname === '/login'),
				page.getByTestId('change-password-submit').click(),
			]);
			await otherPage.goto('/account');
			await expect(otherPage).toHaveURL(/\/login/);

			await page.locator('[type="email"]').fill(email);
			await page.locator('[type="password"]').fill(oldPassword);
			await page.locator('[type="submit"]').click();
			await expect(page).toHaveURL(/\/login/);
			await page.locator('[type="password"]').fill(newPassword);
			await Promise.all([
				page.waitForURL((url) => url.pathname !== '/login'),
				page.locator('[type="submit"]').click(),
			]);

			await page.goto('/account');
			await page.locator('[name="MiddleName"]').fill('Profile edit');
			const profileResponse = page.waitForResponse((response) => response.request().method() === 'POST' && new URL(response.url()).pathname === '/account');
			await page.locator('form[hx-post="/account"] [type="submit"]').click();
			expect((await profileResponse).ok()).toBeTruthy();
			await page.context().clearCookies();
			await login(page, email, newPassword);
		} finally {
			await otherContext.close();
		}
	});
});
