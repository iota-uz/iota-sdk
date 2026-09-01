import { createHash, randomBytes } from 'node:crypto';
import { expect, test, type Page } from '@playwright/test';
import { populateTestData, resetTestDatabase } from '../../fixtures/test-data';

const password = 'TestPass123!';

async function credentialLogin(page: Page, email: string) {
	await page.goto('/login#login-methods');
	await page.locator('form#login-methods [type=email]').fill(email);
	await page.locator('form#login-methods [type=password]').fill(password);
	await Promise.all([
		page.waitForURL((url) => url.pathname !== '/login'),
		page.locator('form#login-methods button[type=submit]').click(),
	]);
}

async function seedAccounts(request: Parameters<typeof resetTestDatabase>[0], count = 6) {
	await resetTestDatabase(request, { reseedMinimal: true });
	await populateTestData(request, {
		version: '1.0',
		tenant: {
			id: '00000000-0000-0000-0000-000000000001',
			name: 'Account Picker Tenant',
			domain: 'localhost',
		},
		data: {
			users: Array.from({ length: count }, (_, index) => ({
				email: `picker-${index + 1}@example.test`,
				password,
				firstName: `Picker ${index + 1}`,
				lastName: 'Account',
				language: 'en',
			})),
		},
	});
}

test.describe.serial('browser account switching', () => {
	test('keeps accounts, switches without credentials, and scopes logout', async ({ page, request }) => {
		await seedAccounts(request, 2);
		await credentialLogin(page, 'picker-1@example.test');
		await credentialLogin(page, 'picker-2@example.test');

		await page.goto('/login');
		await expect(page.getByTestId('account-card')).toHaveCount(2);
		// This would be falsely green if saved sessions replaced the credential
		// form. Both entry paths must remain visible in the same rendered state.
		await expect(page.getByTestId('account-picker')).toBeVisible();
		await expect(page.getByTestId('login-form')).toBeVisible();
		await expect(page.getByTestId('account-login-divider')).toHaveText('Or');
		await expect(page.locator('form#login-methods [type=email]')).toBeVisible();
		const desktopLayout = await page.evaluate(() => {
			const picker = document.querySelector<HTMLElement>('[data-testid="account-picker"]');
			const divider = document.querySelector<HTMLElement>('[data-testid="account-login-divider"]');
			const form = document.querySelector<HTMLElement>('[data-testid="login-form"]');
			return Boolean(
				picker && divider && form &&
				picker.getBoundingClientRect().bottom <= divider.getBoundingClientRect().top &&
				divider.getBoundingClientRect().bottom <= form.getBoundingClientRect().top
			);
		});
		// This would be falsely green if only the mobile breakpoint stacked the
		// choices. Saved sessions stay above the form at desktop widths too.
		expect(desktopLayout).toBe(true);
		await page.setViewportSize({ width: 390, height: 844 });
		const mobileLayout = await page.evaluate(() => {
			const picker = document.querySelector<HTMLElement>('[data-testid="account-picker"]');
			const form = document.querySelector<HTMLElement>('[data-testid="login-form"]');
			return {
				stacked: Boolean(picker && form && picker.getBoundingClientRect().bottom <= form.getBoundingClientRect().top),
				overflows: document.body.scrollWidth > window.innerWidth,
			};
		});
		expect(mobileLayout).toEqual({ stacked: true, overflows: false });
		await page.setViewportSize({ width: 1280, height: 720 });

		await Promise.all([
			page.waitForURL((url) => url.pathname === '/'),
			page.getByTestId('account-card').filter({ hasText: 'picker-1@example.test' }).click(),
		]);

		await page.goto('/login');
		await expect(page.getByTestId('account-card').filter({ hasText: 'picker-1@example.test' })).toContainText('Active');
		await page.goto('/');
		await page.locator('details[name="details-dropdown"] > summary').first().click();
		await Promise.all([
			page.waitForURL((url) => url.pathname === '/'),
			page.locator('form[action="/logout"] button').click(),
		]);

		await page.goto('/login');
		await expect(page.getByTestId('account-card')).toHaveCount(1);
		await expect(page.getByTestId('account-card')).toContainText('picker-2@example.test');

		await page.goto('/');
		await page.locator('details[name="details-dropdown"] > summary').first().click();
		await Promise.all([
			page.waitForURL((url) => url.pathname === '/login'),
			page.locator('form[action="/logout/all"] button').click(),
		]);
		await expect(page.getByTestId('account-picker')).toHaveCount(0);
	});

	test('evicts the least-recently-active account when a sixth account is added', async ({ page, request }) => {
		await seedAccounts(request);
		for (let index = 1; index <= 6; index += 1) {
			await credentialLogin(page, `picker-${index}@example.test`);
		}
		await page.goto('/login');
		await expect(page.getByTestId('account-card')).toHaveCount(5);
		await expect(page.getByTestId('account-card').filter({ hasText: 'picker-1@example.test' })).toHaveCount(0);
	});

	test('OIDC uses the shared picker and preserves state, nonce, and PKCE', async ({ page, request, baseURL }) => {
		expect(baseURL).toBeTruthy();
		const applicationURL = baseURL!;
		const redirectURI = `${applicationURL}/__test__/oidc-callback`;
		await resetTestDatabase(request, { reseedMinimal: true });
		await populateTestData(request, {
			version: '1.0',
			tenant: {
				id: '00000000-0000-0000-0000-000000000001',
				name: 'OIDC Picker Tenant',
				domain: 'localhost',
			},
			data: {
				users: [
					{ email: 'oidc-one@example.test', password, firstName: 'OIDC One', lastName: 'Account', language: 'en' },
					{ email: 'oidc-two@example.test', password, firstName: 'OIDC Two', lastName: 'Account', language: 'en' },
				],
				oidcClients: [{
					clientId: 'account-picker-e2e',
					name: 'Account Picker E2E',
					redirectUris: [redirectURI],
					scopes: ['openid', 'profile', 'email', 'tenant_id'],
				}],
			},
		});
		await credentialLogin(page, 'oidc-one@example.test');
		await credentialLogin(page, 'oidc-two@example.test');

		const verifier = randomBytes(32).toString('base64url');
		const challenge = createHash('sha256').update(verifier).digest('base64url');
		const state = `state-${randomBytes(8).toString('hex')}`;
		const nonce = `nonce-${randomBytes(8).toString('hex')}`;
		const authorize = new URL('/oidc/authorize', applicationURL);
		authorize.search = new URLSearchParams({
			client_id: 'account-picker-e2e',
			redirect_uri: redirectURI,
			response_type: 'code',
			scope: 'openid profile email tenant_id',
			state,
			nonce,
			code_challenge: challenge,
			code_challenge_method: 'S256',
		}).toString();

		await page.goto(authorize.toString());
		await expect(page).toHaveURL(/\/login\?auth_request=/);
		await expect(page.getByTestId('account-card')).toHaveCount(2);
		await Promise.all([
			page.waitForURL((url) => url.pathname === '/__test__/oidc-callback'),
			page.getByTestId('account-card').filter({ hasText: 'oidc-one@example.test' }).click(),
		]);

		const callback = new URL(page.url());
		expect(callback.searchParams.get('state')).toBe(state);
		const code = callback.searchParams.get('code');
		expect(code).toBeTruthy();

		const tokenResponse = await request.post('/oidc/oauth/token', {
			form: {
				grant_type: 'authorization_code',
				client_id: 'account-picker-e2e',
				redirect_uri: redirectURI,
				code: code!,
				code_verifier: verifier,
			},
			failOnStatusCode: false,
		});
		expect(tokenResponse.ok(), await tokenResponse.text()).toBeTruthy();
		const tokens = await tokenResponse.json();
		const claims = JSON.parse(Buffer.from(tokens.id_token.split('.')[1], 'base64url').toString('utf8'));
		expect(claims.nonce).toBe(nonce);
		expect(claims.tenant_id).toBe('00000000-0000-0000-0000-000000000001');

		const userInfoResponse = await request.get('/oidc/userinfo', {
			headers: { Authorization: `Bearer ${tokens.access_token}` },
			failOnStatusCode: false,
		});
		expect(userInfoResponse.ok(), await userInfoResponse.text()).toBeTruthy();
		const userInfo = await userInfoResponse.json();
		expect(userInfo.email).toBe('oidc-one@example.test');
	});
});
