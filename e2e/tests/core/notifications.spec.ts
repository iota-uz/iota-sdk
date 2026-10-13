import { test, expect } from '@playwright/test';
import { login } from '../../fixtures/auth';
import { withDatabase } from '../../fixtures/database';
import { randomUUID } from 'node:crypto';

test('business can configure delivery, receive notifications, read them and disable delivery', async ({ page, context }) => {
	test.setTimeout(90000);
	await login(page, 'test@gmail.com', 'TestPass123!');
 const groupID = randomUUID();
 const roleID = await withDatabase(async (db) => {
  const { rows: [recipient] } = await db.query("SELECT id,tenant_id FROM users WHERE email=$1", ['test@gmail.com']);
  await db.query("INSERT INTO user_groups(id,type,name,tenant_id) VALUES($1,'user',$2,$3)", [groupID, 'Notification pilot group '+groupID, recipient.tenant_id]);
  const { rows: [role] } = await db.query("INSERT INTO roles(type,name,tenant_id) VALUES('user',$1,$2) RETURNING id", ['Notification pilot role '+groupID, recipient.tenant_id]);
  await db.query('INSERT INTO group_users(group_id,user_id) VALUES($1,$2)', [groupID, recipient.id]);
  await db.query('INSERT INTO group_roles(group_id,role_id) VALUES($1,$2)', [groupID, role.id]);
  return role.id;
 });
 await page.goto('/settings/notifications');
	const rule = page.locator('form').filter({ has: page.locator('input[name="event_key"][value="core.notification.test.v1"]') });
	await rule.locator('input[name="enabled"]').check();
	for (const checkbox of await rule.locator('input[name="user_ids"], input[name="group_ids"], input[name="role_ids"], input[name="recipient_keys"]').all()) {
		await checkbox.uncheck();
	}
	await rule.locator('label').filter({ hasText: 'test@gmail.com' }).locator('input').check();
 await rule.locator(`input[name="group_ids"][value="${groupID}"]`).check();
 await rule.locator(`input[name="role_ids"][value="${roleID}"]`).check();
 await rule.locator('select[name="level"]').selectOption('warning');
 await rule.getByRole('button', { name: 'Save rule', exact: true }).click();
	await expect(page.getByRole('status')).toContainText('saved');
	await page.reload();
	await expect(rule.locator('input[name="enabled"]')).toBeChecked();
	await expect(rule.locator('label').filter({ hasText: 'test@gmail.com' }).locator('input')).toBeChecked();
 await expect(rule.locator(`input[name="group_ids"][value="${groupID}"]`)).toBeChecked();
 await expect(rule.locator(`input[name="role_ids"][value="${roleID}"]`)).toBeChecked();
 const inbox = await context.newPage();
	const secondaryOrigin = process.env.SECONDARY_BASE_URL;
 await inbox.goto(secondaryOrigin ? new URL('/notifications?unread=true', secondaryOrigin).href : '/notifications?unread=true');
	await inbox.evaluate(() => { window.addEventListener('notify', (event) => { (window as any).lastNotificationToast = (event as CustomEvent).detail; (window as any).notificationToastCount = ((window as any).notificationToastCount || 0) + 1; }); });
 await inbox.evaluate(async () => {
  await (window as any).htmx.ajax('GET', '/notifications?unread=true', { target: 'body', swap: 'outerHTML', headers: { 'HX-Request': 'false' } });
 });
 const before = await inbox.getByTestId('notification').count();
	await rule.getByRole('button', { name: 'Send test using saved rule', exact: true }).click();
	await expect(page.getByRole('status')).toContainText('1');
	// Every connected tab receives realtime invalidation and the SDK severity toast.
	await expect(inbox.getByTestId('notification')).toHaveCount(before + 1, { timeout: 30000 });
	await expect(inbox.getByTestId('notification-unread-count')).toBeVisible();
 await expect.poll(() => inbox.evaluate(() => (window as any).lastNotificationToast?.variant)).toBe('warning');
 expect(await inbox.evaluate(() => (window as any).notificationToastCount)).toBe(1);
 await inbox.getByRole('button', { name: 'Close notification', exact: true }).click();
 await inbox.getByTestId('notification-bell').locator('summary').click();
 await expect(inbox.getByTestId('notification-dropdown-item').first()).toContainText('Test');
 await inbox.getByTestId('notification-bell').locator('summary').click();
	await inbox.getByTestId('notification').first().getByRole('button', { name: 'Mark read', exact: true }).click();
	await expect(inbox.getByTestId('notification')).toHaveCount(before);
	await inbox.reload();
	await expect(inbox.getByTestId('notification')).toHaveCount(before);
 await rule.locator('input[name="user_ids"]:checked').uncheck();
 await rule.locator(`input[name="group_ids"][value="${groupID}"]`).uncheck();
 await rule.getByRole('button', { name: 'Save rule', exact: true }).click();
 await expect(page.getByRole('status')).toContainText('saved');
 await rule.getByRole('button', { name: 'Send test using saved rule', exact: true }).click();
 await expect(page.getByRole('status')).toContainText('1');
 await expect(inbox.getByTestId('notification')).toHaveCount(before + 1, { timeout: 30000 });
 await withDatabase(async (db) => { await db.query('DELETE FROM group_users WHERE group_id=$1', [groupID]); });
 await rule.getByRole('button', { name: 'Send test using saved rule', exact: true }).click();
 await expect(page.getByRole('status')).toContainText('No notifications');
 await inbox.reload();
 const afterRoleDelivery = await inbox.getByTestId('notification').count();
 expect(afterRoleDelivery).toBe(before + 1);
 await rule.locator('input[name="enabled"]').uncheck();
	await rule.getByRole('button', { name: 'Save rule', exact: true }).click();
	await expect(page.getByRole('status')).toContainText('saved');
	await rule.getByRole('button', { name: 'Send test using saved rule', exact: true }).click();
	await expect(page.getByRole('status')).toContainText('No notifications');
	await inbox.reload();
	await expect(inbox.getByTestId('notification')).toHaveCount(afterRoleDelivery);
 await withDatabase(async (db) => {
  await db.query('DELETE FROM user_groups WHERE id=$1', [groupID]);
  await db.query('DELETE FROM roles WHERE id=$1', [roleID]);
 });
});
