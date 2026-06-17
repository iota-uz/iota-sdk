import { test, expect } from '@playwright/test';
import { login, logout } from '../../fixtures/auth';
import { resetTestDatabase, seedScenario } from '../../fixtures/test-data';

/**
 * System info page contract tests.
 *
 * Verifies that /system/info renders and reflects the lazy-loading
 * framework's CapabilityRegistry state. The default e2e environment
 * supplies OIDC credentials and leaves bichat/twilio/meili/smtp/loki/
 * redis/googleoauth unset, so we can assert the expected state matrix.
 */
test.describe('system info capabilities panel', () => {
	test.beforeAll(async ({ request }) => {
		await resetTestDatabase(request, { reseedMinimal: false });
		await seedScenario(request, 'comprehensive');
	});

	test.beforeEach(async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 720 });
		await login(page, 'test@gmail.com', 'TestPass123!');
	});

	test.afterEach(async ({ page }) => {
		await logout(page);
	});

	test('renders page with capabilities section', async ({ page }) => {
		await page.goto('/system/info');
		await expect(page).toHaveURL(/\/system\/info/);
		await expect(page.getByRole('heading', { name: /capabilities/i })).toBeVisible();
	});

	test('OIDC capability shows enabled when credentials supplied', async ({ page }) => {
		// .env.e2e sets OIDC_ISSUERURL + OIDC_CRYPTOKEY, so the gate registers
		// oidc as Active. The probe emitted via SkipIfDisabled reports
		// Enabled=true + Status=healthy.
		await page.goto('/system/info');
		const oidcCard = page.locator('[data-capability="oidc"]');
		await expect(oidcCard).toBeVisible();
		await expect(oidcCard.getByText('enabled', { exact: true })).toBeVisible();
		await expect(oidcCard.getByText('healthy', { exact: true })).toBeVisible();
	});

	test('BiChat capability shows disabled when no OpenAI key', async ({ page }) => {
		// Default e2e env has no BICHAT_OPENAI_APIKEY, so the gate reports
		// Disabled with the DisabledReason carried into capability.Message.
		await page.goto('/system/info');
		const bichatCard = page.locator('[data-capability="bichat"]');
		await expect(bichatCard).toBeVisible();
		await expect(bichatCard.getByText('disabled', { exact: true }).first()).toBeVisible();
		await expect(bichatCard).toContainText('BICHAT_OPENAI_APIKEY required');
	});

	test('unconfigured optional features all report disabled', async ({ page }) => {
		// Bichat, twilio, meili, smtp, googleoauth, redis have no credentials
		// in the e2e env → each must show a disabled badge.
		await page.goto('/system/info');
		for (const key of ['bichat', 'twilio', 'meili', 'smtp', 'googleoauth', 'redis']) {
			const card = page.locator(`[data-capability="${key}"]`);
			await expect(card, `${key} capability card missing`).toBeVisible();
			await expect(
				card.getByText('disabled', { exact: true }).first(),
				`${key} should have disabled badge`,
			).toBeVisible();
		}
	});

	test('capability source provenance shown when known', async ({ page }) => {
		// OIDC comes from the env provider (env:.env,.env.local). The
		// provenance line is rendered in monospace under the capability name.
		await page.goto('/system/info');
		const oidcCard = page.locator('[data-capability="oidc"]');
		const source = oidcCard.locator('[data-capability-source]');
		await expect(source).toBeVisible();
		await expect(source).toContainText(/^env/);
	});
});
