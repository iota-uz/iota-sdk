/**
 * Onboarding fixtures: accounts created through /users/new start in
 * pending_onboarding and must set their own password before they can use the app.
 */

import { expect, Page } from '@playwright/test';
import { login } from './auth';

export interface OnboardingProfile {
	firstName: string;
	lastName: string;
	newPassword: string;
	middleName?: string;
	/** Language code; defaults to 'en' because /users/new offers languages the app may not enable. */
	language?: string;
}

/**
 * Submits /users/new (the form must already be filled) and returns the
 * temporary password shown once in the created-user fragment.
 */
export async function submitNewUserForm(page: Page): Promise<string> {
	await page.locator('#save-btn').click();
	await expect(page.getByTestId('user-created')).toBeVisible();
	const temporaryPassword = (await page.getByTestId('temporary-password-value').textContent())?.trim() ?? '';
	expect(temporaryPassword.length).toBeGreaterThanOrEqual(8);
	return temporaryPassword;
}

/**
 * Fills and submits the onboarding form. Expects the page to be on /onboarding
 * and leaves it on /login with the success notice.
 */
export async function completeOnboarding(page: Page, profile: OnboardingProfile) {
	await expect(page).toHaveURL(/\/onboarding$/);
	const form = page.getByTestId('onboarding-form');
	await page.getByTestId('onboarding-language').selectOption(profile.language ?? 'en');
	await form.locator('[name=FirstName]').fill(profile.firstName);
	await form.locator('[name=LastName]').fill(profile.lastName);
	if (profile.middleName !== undefined) {
		await form.locator('[name=MiddleName]').fill(profile.middleName);
	}
	await page.getByTestId('onboarding-new-password').fill(profile.newPassword);
	await page.getByTestId('onboarding-confirm-password').fill(profile.newPassword);
	await Promise.all([
		page.waitForURL((url) => url.pathname === '/login'),
		page.getByTestId('onboarding-submit').click(),
	]);
	await expect(page.getByTestId('login-notice')).toBeVisible();
}

/**
 * Signs in with the temporary password, completes onboarding and signs in
 * again with the new password.
 */
export async function loginThroughOnboarding(
	page: Page,
	email: string,
	temporaryPassword: string,
	profile: OnboardingProfile,
) {
	await login(page, email, temporaryPassword);
	await completeOnboarding(page, profile);
	await login(page, email, profile.newPassword);
}
