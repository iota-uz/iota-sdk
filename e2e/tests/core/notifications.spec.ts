import { test, expect } from '@playwright/test';
import { login } from '../../fixtures/auth';

test('business can configure delivery, receive notifications, read them and disable delivery', async ({ page, context }) => {
	test.setTimeout(90000);
	await login(page, 'test@gmail.com', 'TestPass123!');
	await page.goto('/settings/notifications');
	const rule = page.locator('form').filter({ has: page.locator('input[name="event_key"][value="core.notification.test.v1"]') });
	await rule.locator('input[name="enabled"]').check();
	await rule.locator('label').filter({ hasText: 'test@gmail.com' }).locator('input').check();
	await rule.getByRole('button', { name: 'Save rule', exact: true }).click();
	await expect(page.getByRole('status')).toContainText('saved');
	await page.reload();
	await expect(rule.locator('input[name="enabled"]')).toBeChecked();
	await expect(rule.locator('label').filter({ hasText: 'test@gmail.com' }).locator('input')).toBeChecked();
	const inbox = await context.newPage();
	await inbox.goto('/notifications?unread=true');
	const before = await inbox.getByTestId('notification').count();
	await rule.getByRole('button', { name: 'Send test using saved rule', exact: true }).click();
	await expect(page.getByRole('status')).toContainText('1');
	// A second tab must discover database changes without a shared WebSocket process.
	await expect(inbox.getByTestId('notification')).toHaveCount(before + 1, { timeout: 30000 });
	await expect(inbox.getByTestId('notification-unread-count')).toBeVisible();
	await inbox.getByTestId('notification').first().getByRole('button', { name: 'Mark read', exact: true }).click();
	await expect(inbox.getByTestId('notification')).toHaveCount(before);
	await inbox.reload();
	await expect(inbox.getByTestId('notification')).toHaveCount(before);
	await rule.locator('input[name="enabled"]').uncheck();
	await rule.getByRole('button', { name: 'Save rule', exact: true }).click();
	await expect(page.getByRole('status')).toContainText('saved');
	await rule.getByRole('button', { name: 'Send test using saved rule', exact: true }).click();
	await expect(page.getByRole('status')).toContainText('No notifications');
	await inbox.reload();
	await expect(inbox.getByTestId('notification')).toHaveCount(before);
});
