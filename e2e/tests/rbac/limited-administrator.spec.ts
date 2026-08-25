import { expect, test, type Page } from '@playwright/test';
import { login, logout, populateTestData, resetTestDatabase, seedScenario, waitForAlpine } from '../../fixtures';

const limitedAdmin = {
	email: 'limited-administrator@test.com',
	password: 'LimitedAdminPass123!',
};

const permissions = {
	userRead: '13f011c8-1107-4957-ad19-70cfc167a775',
	groupRead: '8f9a0b1c-2d3e-4f5a-6b7c-8d9e0f1a2b3c',
};

const limitedAdminPermissionNames = [
	'User.Create',
	'User.Read',
	'User.Update',
	'User.Delete',
	'User.UpdateBlockStatus',
	'Role.Create',
	'Role.Read',
	'Role.Update',
	'Role.Delete',
	'Group.Create',
	'Group.Read',
	'Group.Update',
	'Group.Delete',
];

async function expectNotAdmin(page: Page): Promise<void> {
	const response = await page.request.get('/departments', { failOnStatusCode: false });
	expect(response.status(), 'the limited administrator must not inherit Admin').toBe(403);
}

async function selectOptionsByLabel(page: Page, selector: string, labels: string[]): Promise<void> {
	await page.locator(selector).evaluate((element, selectedLabels) => {
		if (!(element instanceof HTMLSelectElement)) throw new Error('expected a select element');

		const wanted = new Set(selectedLabels);
		let matched = 0;
		for (const option of Array.from(element.options)) {
			option.selected = wanted.has(option.text.trim());
			if (option.selected) matched++;
		}
		if (matched !== wanted.size) {
			throw new Error(`expected ${wanted.size} options, matched ${matched}`);
		}
		element.dispatchEvent(new Event('input', { bubbles: true }));
		element.dispatchEvent(new Event('change', { bubbles: true }));
	}, labels);
}

async function setCheckboxState(page: Page, selector: string, checked: boolean): Promise<void> {
	await page.locator(selector).first().evaluate((element, nextChecked) => {
		if (!(element instanceof HTMLInputElement)) throw new Error('expected a checkbox input');
		element.checked = nextChecked;
		element.dispatchEvent(new Event('change', { bubbles: true }));
	}, checked);
}

async function createRole(page: Page, name: string, permissionIDs: string[]): Promise<string> {
	await page.goto('/roles/new');
	await expect(page).toHaveURL(/\/roles\/new$/);
	await waitForAlpine(page);
	await page.locator('[data-test-id="role-name-input"]').fill(name);
	await page.locator('[data-test-id="role-description-input"]').fill(`${name} description`);

	for (const permissionID of permissionIDs) {
		const input = page.locator(`input[name="Permissions[${permissionID}]"]`).first();
		await expect(input).toBeAttached();
		await setCheckboxState(page, `input[name="Permissions[${permissionID}]"]`, true);
	}

	await page.locator('[data-test-id="save-role-btn"]').click();
	await page.waitForURL(/\/roles$/);

	const row = page.locator('tbody tr').filter({ hasText: name });
	await expect(row).toBeVisible();
	const href = await row.locator('a[href^="/roles/"]').first().getAttribute('href');
	const match = href?.match(/\/roles\/(\d+)$/);
	if (!match) throw new Error(`role edit URL was not rendered for ${name}`);
	return match[1];
}

async function createUser(
	page: Page,
	data: {
		firstName: string;
		lastName: string;
		email: string;
		phone: string;
		password: string;
		roleNames?: string[];
		groupNames?: string[];
	},
): Promise<string> {
	await page.goto('/users/new');
	await expect(page).toHaveURL(/\/users\/new$/);
	await page.locator('[name=FirstName]').fill(data.firstName);
	await page.locator('[name=LastName]').fill(data.lastName);
	await page.locator('[name=Email]').fill(data.email);
	await page.locator('[name=Phone]').fill(data.phone);
	await page.locator('[name=Password]').fill(data.password);
	await page.locator('[name=Language]').selectOption('en');

	if (data.roleNames) await selectOptionsByLabel(page, 'select[name="RoleIDs"]', data.roleNames);
	if (data.groupNames) await selectOptionsByLabel(page, 'select[name="GroupIDs"]', data.groupNames);

	await page.locator('#save-btn').click();
	await page.waitForURL(/\/users$/);

	const row = page.locator('tbody tr').filter({ hasText: `${data.firstName} ${data.lastName}` });
	await expect(row).toBeVisible();
	const href = await row.locator('a[href$="/edit"]').first().getAttribute('href');
	const match = href?.match(/\/users\/(\d+)\/edit$/);
	if (!match) throw new Error(`user edit URL was not rendered for ${data.email}`);
	return match[1];
}

async function submitDeleteFormViaHTMX(page: Page): Promise<void> {
	const result = await page.evaluate(async () => {
		const form = document.getElementById('delete-form');
		if (!(form instanceof HTMLFormElement)) throw new Error('delete form was not rendered');
		const endpoint = form.getAttribute('hx-delete');
		if (!endpoint) throw new Error('delete endpoint was not rendered');

		const params = new URLSearchParams();
		for (const [key, value] of new FormData(form).entries()) params.append(key, String(value));

		const response = await fetch(endpoint, {
			method: 'DELETE',
			credentials: 'same-origin',
			headers: {
				'HX-Request': 'true',
				'X-Requested-With': 'XMLHttpRequest',
				'Content-Type': 'application/x-www-form-urlencoded; charset=UTF-8',
			},
			body: params.toString(),
		});
		return {
			status: response.status,
			redirect: response.headers.get('HX-Redirect') ?? response.headers.get('Location'),
		};
	});

	expect(result.status).toBeLessThan(400);
	if (result.redirect) await page.goto(result.redirect);
}

test.describe('limited administrator P0 management flows', () => {
	test.describe.configure({ mode: 'serial' });

	test.beforeAll(async ({ request }) => {
		await resetTestDatabase(request, { reseedMinimal: false });
		await seedScenario(request, 'comprehensive');
		await populateTestData(request, {
			version: '1.0',
			tenant: {
				id: '00000000-0000-0000-0000-000000000001',
				name: 'Default Test Tenant',
				domain: 'test.localhost',
			},
			data: {
				users: [{
					email: limitedAdmin.email,
					password: limitedAdmin.password,
					firstName: 'Limited',
					lastName: 'Administrator',
					language: 'en',
					permissions: limitedAdminPermissionNames,
				}],
			},
			options: { clearExisting: false, returnIds: true, stopOnError: true },
		});
	});

	test.beforeEach(async ({ page }) => {
		await login(page, limitedAdmin.email, limitedAdmin.password);
		await expectNotAdmin(page);
	});

	test.afterEach(async ({ page }) => {
		await logout(page);
	});

	test('creates, updates, and deletes a role inside the grant ceiling', async ({ page }) => {
		const originalName = 'P0 Limited Role';
		const updatedName = 'P0 Limited Role Updated';
		const roleID = await createRole(page, originalName, [permissions.userRead]);

		await page.goto(`/roles/${roleID}`);
		await page.locator('[data-test-id="role-name-input"]').fill(updatedName);
		await page.locator('[data-test-id="role-description-input"]').fill('Updated inside the actor ceiling');
		await setCheckboxState(page, `input[name="Permissions[${permissions.groupRead}]"]`, true);
		await page.locator('[data-test-id="save-role-btn"]').click();
		await page.waitForURL(/\/roles$/);

		const updatedRow = page.locator('tbody tr').filter({ hasText: updatedName });
		await expect(updatedRow).toBeVisible();
		await page.goto(`/roles/${roleID}`);
		await expect(page.locator(`input[name="Permissions[${permissions.userRead}]"]`).first()).toBeChecked();
		await expect(page.locator(`input[name="Permissions[${permissions.groupRead}]"]`).first()).toBeChecked();

		await page.locator('[data-test-id="delete-role-btn"]').click();
		await submitDeleteFormViaHTMX(page);
		await expect(page).toHaveURL(/\/roles$/);
		await expect(page.locator('tbody tr').filter({ hasText: updatedName })).toHaveCount(0);

		// Falsely green if the actor is Admin: expectNotAdmin proves the same
		// session cannot access an unrelated resource before exercising CRUD.
	});

	test('manages the full lifecycle of a weaker user', async ({ page }) => {
		const roleName = 'P0 Managed User Reader';
		const roleID = await createRole(page, roleName, [permissions.userRead]);
		const userID = await createUser(page, {
			firstName: 'P0Managed',
			lastName: 'User',
			email: 'p0-managed-user@test.com',
			phone: '+998901112201',
			password: 'ManagedUserPass123!',
			roleNames: [roleName],
		});

		await page.goto(`/users/${userID}/edit`);
		await page.locator('[name=FirstName]').fill('P0Updated');
		await page.locator('#save-btn').click();
		await page.waitForURL(/\/users$/);
		await expect(page.locator('tbody tr').filter({ hasText: 'P0Updated User' })).toBeVisible();

		await page.goto(`/users/${userID}/edit`);
		await page.getByRole('button', { name: 'Block User', exact: true }).click();
		const blockDrawer = page.locator(`#block-user-drawer-${userID}`);
		await expect(blockDrawer.locator('[name=BlockReason]')).toBeVisible();
		await blockDrawer.locator('[name=BlockReason]').fill('P0 lifecycle verification');
		await blockDrawer.getByRole('button', { name: 'Block User', exact: true }).click();
		await expect(page.getByText('Blocked', { exact: true })).toBeVisible();

		await expect(page.getByRole('button', { name: 'Unblock User', exact: true })).toBeVisible();
		const unblockResponse = await page.request.post(`/users/${userID}/unblock`, {
			headers: { 'HX-Request': 'true' },
			failOnStatusCode: false,
		});
		expect(unblockResponse.status()).toBe(200);
		await page.reload();
		await expect(page.getByRole('button', { name: 'Block User', exact: true })).toBeVisible();

		await page.locator('#delete-user-btn').click();
		await submitDeleteFormViaHTMX(page);
		await expect(page).toHaveURL(/\/users$/);
		await expect(page.locator('tbody tr').filter({ hasText: 'P0Updated User' })).toHaveCount(0);

		await page.goto(`/roles/${roleID}`);
		await page.locator('[data-test-id="delete-role-btn"]').click();
		await submitDeleteFormViaHTMX(page);

		// Falsely green if only creation works: the same persisted target is
		// updated, blocked, unblocked, and deleted through its real UI routes.
	});

	test('propagates and revokes access through a group role', async ({ page }) => {
		const roleName = 'P0 Group User Reader';
		const groupName = 'P0 Permission Group';
		const updatedGroupName = 'P0 Permission Group Updated';
		const targetEmail = 'p0-group-user@test.com';
		const targetPassword = 'GroupUserPass123!';
		const roleID = await createRole(page, roleName, [permissions.userRead]);

		await page.goto('/groups');
		await page.locator('button:visible').filter({ hasText: 'New group' }).click();
		const newDrawer = page.locator('#new-group-drawer');
		await expect(newDrawer.locator('form[hx-post="/groups"]')).toBeVisible();
		await newDrawer.locator('[name=Name]').fill(groupName);
		await setCheckboxState(page, `#new-group-drawer input[name=RoleIDs][value="${roleID}"]`, true);
		await newDrawer.locator('#save-btn').click();
		await page.waitForURL(/\/groups$/);

		let groupRow = page.locator('#groups-table-body tr').filter({ hasText: groupName });
		await expect(groupRow).toBeVisible();
		const rowID = await groupRow.getAttribute('id');
		const groupID = rowID?.replace(/^group-/, '');
		if (!groupID) throw new Error('created group ID was not rendered');

		await groupRow.click();
		const editDrawer = page.locator('#edit-group-drawer');
		await expect(editDrawer.locator('[name=Name]')).toBeVisible();
		await editDrawer.locator('[name=Name]').fill(updatedGroupName);
		await editDrawer.locator('#save-btn').click();
		await page.waitForURL(/\/groups$/);
		groupRow = page.locator('#groups-table-body tr').filter({ hasText: updatedGroupName });
		await expect(groupRow).toBeVisible();

		const userID = await createUser(page, {
			firstName: 'P0Group',
			lastName: 'User',
			email: targetEmail,
			phone: '+998901112202',
			password: targetPassword,
			groupNames: [updatedGroupName],
		});

		await logout(page);
		await login(page, targetEmail, targetPassword);
		const inheritedAccess = await page.goto('/users');
		expect(inheritedAccess?.status()).toBe(200);
		await expect(page.locator('table')).toBeVisible();

		await logout(page);
		await login(page, limitedAdmin.email, limitedAdmin.password);
		await page.goto(`/users/${userID}/edit`);
		await selectOptionsByLabel(page, 'select[name="GroupIDs"]', []);
		await page.locator('#save-btn').click();
		await page.waitForURL(/\/users$/);

		await logout(page);
		await login(page, targetEmail, targetPassword);
		const revokedAccess = await page.request.get('/users', { failOnStatusCode: false });
		expect(revokedAccess.status()).toBe(403);

		await logout(page);
		await login(page, limitedAdmin.email, limitedAdmin.password);
		await page.goto(`/users/${userID}/edit`);
		await page.locator('#delete-user-btn').click();
		await submitDeleteFormViaHTMX(page);

		await page.goto('/groups');
		groupRow = page.locator('#groups-table-body tr').filter({ hasText: updatedGroupName });
		await groupRow.click();
		await page.locator('#edit-group-drawer #delete-group-btn').click();
		await page.waitForURL(/\/groups$/);
		await expect(page.locator('#groups-table-body tr').filter({ hasText: updatedGroupName })).toHaveCount(0);

		await page.goto(`/roles/${roleID}`);
		await page.locator('[data-test-id="delete-role-btn"]').click();
		await submitDeleteFormViaHTMX(page);

		// Falsely green if the target receives User.Read directly or through a
		// role: removing its only group must turn the same /users request to 403.
	});
});
