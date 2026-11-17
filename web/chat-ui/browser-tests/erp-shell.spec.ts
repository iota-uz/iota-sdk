import { expect, test } from '@playwright/test';

// Falsely green if asserted against a stub: the fixture mounts the public package with the real RU locale inside an ERP-like shell.
const mobile = { width: 390, height: 844 };

test('mobile welcome screen keeps the composer at the bottom', async ({ page }) => {
 await page.setViewportSize(mobile);
 await page.goto('/erp.html');
 const composer = page.getByRole('textbox').first();
 await expect(composer).toBeVisible();
 const box = (await composer.boundingBox())!;
 expect(box.y + box.height).toBeGreaterThan(mobile.height - 140);
 await expect(page.getByRole('button', { name: 'Открыть боковую панель' })).toHaveCount(1);
});

test('desktop welcome screen stays centered', async ({ page }) => {
 await page.setViewportSize({ width: 1280, height: 900 });
 await page.goto('/erp.html');
 const box = (await page.getByRole('textbox').first().boundingBox())!;
 expect(box.y + box.height).toBeLessThan(900 - 140);
});

for (const viewport of [mobile, { width: 1280, height: 900 }]) {
 test(`stream error is translated and shown with the failed message at ${viewport.width}px`, async ({ page }) => {
  await page.setViewportSize(viewport);
  await page.goto('/erp.html?session=1');
  await page.getByRole('textbox').first().fill('вопрос');
  await page.keyboard.press('Enter');
  const error = page.getByTestId('bichat-turn-error');
  await expect(error).toBeVisible();
  await expect(error).toContainText('Не удалось получить ответ');
  await expect(page.getByText('stream_failed')).toHaveCount(0);
  await expect(page.getByText('An error occurred')).toHaveCount(0);
  await expect(page.getByTestId('bichat-message-list').getByTestId('bichat-turn-error')).toHaveCount(1);
  await expect(page.getByRole('alert')).toHaveCount(1);
 });
}

test('owner without author data gets the viewer initials, never U', async ({ page }) => {
 await page.setViewportSize(mobile);
 await page.goto('/erp.html?session=1&nameless=1');
 const list = page.getByTestId('bichat-message-list');
 await expect(list.getByText('МФ', { exact: true })).toBeVisible();
 await expect(list.getByText('U', { exact: true })).toHaveCount(0);
});
