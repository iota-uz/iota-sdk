import { test, expect, type Page } from '@playwright/test';
import { login, logout } from '../../fixtures/auth';
import { resetTestDatabase, seedScenario } from '../../fixtures/test-data';
import { withDatabase } from '../../fixtures/database';

async function createEmployee(
	page: Page,
	data: { firstName: string; email: string; phone: string; resignationDate?: string }
): Promise<void> {
	await page.goto('/hrm/employees/new');
	await page.locator('[name=FirstName]').fill(data.firstName);
	await page.locator('[name=LastName]').fill('Status');
	await page.locator('[name=Email]').fill(data.email);
	await page.locator('[name=Phone]').fill(data.phone);
	await page.locator('[name=HireDate]').fill('2024-01-10');
	if (data.resignationDate) {
		await page.locator('[name=ResignationDate]').fill(data.resignationDate);
	}
	await page.locator('[data-tab-value="private"]').click();
	await page.locator('[name=Salary]').fill('1000');
	await page.locator('#save-btn').click();
	await expect(page).toHaveURL(/\/hrm\/employees$/);
}

function employeeRow(page: Page, firstName: string) {
	return page.locator('#employees-table tbody tr', { hasText: firstName });
}

test.describe('employees CRUD operations', () => {
	test.beforeAll(async ({ request }) => {
		// Reset database and seed with comprehensive data for employee management
		await resetTestDatabase(request, { reseedMinimal: false });
		await seedScenario(request, 'comprehensive');
		// The reset truncates currencies, and HRM stores every salary in USD.
		await withDatabase(db =>
			db.query(`INSERT INTO currencies (code, name, symbol) VALUES ('USD', 'US Dollar', '$') ON CONFLICT (code) DO NOTHING`)
		);
	});

	test.beforeEach(async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 720 });
	});

	test.afterEach(async ({ page }) => {
		await logout(page);
	});

	test('displays employees list page', async ({ page }) => {
		await login(page, 'test@gmail.com', 'TestPass123!');

		await page.goto('/hrm/employees');
		await expect(page).toHaveURL(/\/hrm\/employees$/);

		// Check page title and main elements
		await expect(page.locator('h1')).toContainText('Employees');
		await expect(page.locator('a[href="/hrm/employees/new"]')).toBeVisible();

		// Check search and filter form
		await expect(page.locator('form input[name="name"]')).toBeVisible();
		await expect(page.locator('form select[name="limit"]')).toBeVisible();
	});

	test('separates active and former employees by resignation date', async ({ page }) => {
		await login(page, 'test@gmail.com', 'TestPass123!');

		await createEmployee(page, { firstName: 'Alisa', email: 'alisa.status@example.com', phone: '+998901110001' });
		await createEmployee(page, {
			firstName: 'Bobur',
			email: 'bobur.status@example.com',
			phone: '+998901110002',
			resignationDate: '2026-09-01',
		});

		await expect(employeeRow(page, 'Alisa')).toContainText('Active');
		await expect(employeeRow(page, 'Bobur')).toContainText('Former');

		await page.locator('form select[name="status"]').selectOption('former');
		await expect(employeeRow(page, 'Alisa')).toHaveCount(0);
		await expect(employeeRow(page, 'Bobur')).toBeVisible();

		await page.locator('form select[name="status"]').selectOption('active');
		await expect(employeeRow(page, 'Bobur')).toHaveCount(0);
		await expect(employeeRow(page, 'Alisa')).toBeVisible();

		await employeeRow(page, 'Alisa').locator('a[href^="/hrm/employees/"]').click();
		await expect(page.getByTestId('employee-status')).toContainText('Active');
		await page.locator('[name=ResignationDate]').fill('2026-09-15');
		await page.locator('#save-btn').click();
		await expect(page).toHaveURL(/\/hrm\/employees$/);
		await expect(employeeRow(page, 'Alisa')).toContainText('Former');

		await employeeRow(page, 'Alisa').locator('a[href^="/hrm/employees/"]').click();
		await expect(page.getByTestId('employee-status')).toContainText('Former');
		await expect(page.locator('[name=ResignationDate]')).toHaveValue('2026-09-15');
		await expect(page.locator('[name=HireDate]')).toHaveValue('2024-01-10');
	});
});
