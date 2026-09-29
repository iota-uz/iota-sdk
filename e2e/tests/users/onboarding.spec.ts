import { test, expect, type Page } from '@playwright/test';
import { login, logout } from '../../fixtures/auth';
import { completeOnboarding, submitNewUserForm } from '../../fixtures/onboarding';
import { resetTestDatabase } from '../../fixtures/test-data';
import { withDatabase } from '../../fixtures/database';

const admin = { email: 'test@gmail.com', password: 'TestPass123!' };
const newcomer = {
	email: 'onboarding-newcomer@example.test',
	firstName: 'Onboard',
	lastName: 'Newcomer',
	password: 'NewcomerPass123!',
};
const roleName = 'Onboarding Reader';

async function submitLoginExpectingFailure(page: Page, email: string, password: string) {
	await page.goto('/login');
	await page.fill('[type=email]', email);
	await page.fill('[type=password]', password);
	await Promise.all([
		page.waitForResponse((response) => response.request().method() === 'POST' && new URL(response.url()).pathname === '/login'),
		page.click('[type=submit]'),
	]);
	await expect(page.getByTestId('login-form').getByText('Email or password is invalid')).toBeVisible();
	await expect(page).toHaveURL(/\/login(\?.*)?$/);
}

test.describe('user onboarding with a self-set password', () => {
	test.describe.configure({ mode: 'serial' });

	let roleID = '';
	let userID = 0;
	let temporaryPassword = '';

	test.beforeAll(async ({ request }) => {
		await resetTestDatabase(request, { reseedMinimal: true });
		roleID = await withDatabase(async (db) => {
			const adminRow = await db.query(`SELECT tenant_id FROM users WHERE email = $1 LIMIT 1`, [admin.email]);
			const tenantID = adminRow.rows[0].tenant_id as string;
			const role = await db.query(
				`INSERT INTO roles (type, tenant_id, name, description, created_at, updated_at)
				 VALUES ('user', $1, $2, 'Reads users', NOW(), NOW()) RETURNING id`,
				[tenantID, roleName],
			);
			const permission = await db.query(`SELECT id FROM permissions WHERE name = 'User.Read' LIMIT 1`);
			if (permission.rows.length === 0) throw new Error('User.Read permission is not seeded');
			await db.query(`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`, [role.rows[0].id, permission.rows[0].id]);
			return String(role.rows[0].id);
		});
	});

	test.beforeEach(async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 720 });
	});

	test('admin creates a user from email and role only and sees a generated temporary password', async ({ page }) => {
		// Falsely green if the fragment echoed a submitted password: the field is left empty, so the value must be server-generated.
		await login(page, admin.email, admin.password);
		await page.goto('/users/new');
		await expect(page).toHaveURL(/\/users\/new$/);

		await page.locator('[name=Email]').fill(newcomer.email);
		await page.locator('select[name="RoleIDs"]').selectOption(roleID, { force: true });
		await expect(page.getByTestId('temporary-password-input')).toHaveValue('');
		await expect(page.locator('[name=FirstName]')).toHaveValue('');
		await expect(page.locator('[name=LastName]')).toHaveValue('');

		temporaryPassword = await submitNewUserForm(page);
		expect(temporaryPassword).toMatch(/^[A-Za-z0-9]{16}$/);
		await expect(page).toHaveURL(/\/users\/new$/);
		await expect(page.getByTestId('temporary-password-result')).toContainText(newcomer.email);
		await expect(page.getByTestId('user-created').locator('a[href="/users"]')).toBeVisible();

		userID = await withDatabase(async (db) => {
			const row = await db.query(`SELECT id FROM users WHERE email = $1`, [newcomer.email]);
			return row.rows[0].id as number;
		});
		await page.goto(`/users/${userID}/edit`);
		await expect(page.getByTestId('pending-onboarding-notice')).toBeVisible();
		await expect(page.locator('[name=Password]')).toHaveCount(0);

		await logout(page);
	});

	test('pending user is confined to onboarding until they set their own password', async ({ page }) => {
		// Falsely green if the redirects came from a missing session: /onboarding itself renders the user's email, which needs the pending session.
		await test.step('temporary password opens onboarding', async () => {
			await login(page, newcomer.email, temporaryPassword);
			await expect(page).toHaveURL(/\/onboarding$/);
			await expect(page.getByTestId('onboarding-email')).toHaveValue(newcomer.email);
		});

		await test.step('page navigation is sent back to onboarding', async () => {
			await page.goto('/users');
			await expect(page).toHaveURL(/\/onboarding$/);
			for (const path of ['/users', '/', '/account']) {
				const response = await page.request.get(path);
				expect(new URL(response.url()).pathname, `GET ${path} while pending`).toBe('/onboarding');
			}
		});

		await test.step('reusing the temporary password is rejected on the new password field', async () => {
			await page.goto('/onboarding');
			await page.getByTestId('onboarding-language').selectOption('en');
			await page.locator('[name=FirstName]').fill(newcomer.firstName);
			await page.locator('[name=LastName]').fill(newcomer.lastName);
			await page.getByTestId('onboarding-new-password').fill(temporaryPassword);
			await page.getByTestId('onboarding-confirm-password').fill(temporaryPassword);
			const [response] = await Promise.all([
				page.waitForResponse((r) => r.request().method() === 'POST' && new URL(r.url()).pathname === '/onboarding'),
				page.getByTestId('onboarding-submit').click(),
			]);
			expect(response.status()).toBe(422);
			await expect(page).toHaveURL(/\/onboarding$/);
			const newPasswordID = await page.getByTestId('onboarding-new-password').getAttribute('id');
			await expect(page.locator(`[data-testid="field-error"][data-field-id="${newPasswordID}"]`)).toBeVisible();
			await expect(page.getByTestId('field-error')).toHaveCount(1);
		});

		await test.step('completing onboarding ends the session and returns to login with a notice', async () => {
			await completeOnboarding(page, {
				language: 'en',
				firstName: newcomer.firstName,
				lastName: newcomer.lastName,
				newPassword: newcomer.password,
			});
			expect(new URL(page.url()).searchParams.get('email')).toBe(newcomer.email);
			await page.goto('/users');
			await expect(page).toHaveURL(/\/login/);
		});

		await test.step('the temporary password no longer works and the new one opens the app', async () => {
			await submitLoginExpectingFailure(page, newcomer.email, temporaryPassword);
			await login(page, newcomer.email, newcomer.password);
			await expect(page).not.toHaveURL(/\/onboarding/);
			await page.goto('/users');
			await expect(page).toHaveURL(/\/users$/);
			await expect(page.locator('tbody tr').filter({ hasText: `${newcomer.firstName} ${newcomer.lastName}` })).toBeVisible();
		});

		await logout(page);
	});

	test('reissuing a temporary password revokes sessions and returns an active user to onboarding', async ({ page, browser }) => {
		// Falsely green if the user's session were never live: it opens /users before the reissue and must lose it afterwards.
		const userContext = await browser.newContext();
		const userPage = await userContext.newPage();
		try {
			await login(userPage, newcomer.email, newcomer.password);
			await userPage.goto('/users');
			await expect(userPage).toHaveURL(/\/users$/);

			await login(page, admin.email, admin.password);
			await page.goto(`/users/${userID}/edit`);
			await expect(page.getByTestId('pending-onboarding-notice')).toHaveCount(0);
			page.once('dialog', (dialog) => dialog.accept());
			await page.getByTestId('issue-temporary-password').click();
			const issued = page.locator('#temporary-password-result').getByTestId('temporary-password-value');
			await expect(issued).toBeVisible();
			const reissuedPassword = ((await issued.textContent()) ?? '').trim();
			expect(reissuedPassword).toMatch(/^[A-Za-z0-9]{16}$/);
			expect(reissuedPassword).not.toBe(temporaryPassword);

			await page.reload();
			await expect(page.getByTestId('pending-onboarding-notice')).toBeVisible();

			await userPage.goto('/users');
			await expect(userPage).toHaveURL(/\/login/);
			await userContext.clearCookies();

			await submitLoginExpectingFailure(userPage, newcomer.email, newcomer.password);
			await login(userPage, newcomer.email, reissuedPassword);
			await expect(userPage).toHaveURL(/\/onboarding$/);
		} finally {
			await userContext.close();
		}

		await logout(page);
	});
});
