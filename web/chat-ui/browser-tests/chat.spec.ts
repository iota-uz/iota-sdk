import {expect,test} from '@playwright/test';
// Falsely green if the machine alone works: exercise the public built package and browser composer, cancel and owner disposal.
test('success renders streamed markdown',async({page})=>{
 await page.goto('/');await page.getByRole('textbox',{name:'Message',exact:true}).fill('hello');await page.getByRole('button',{name:'Send',exact:true}).click();
 await expect(page.locator('strong').filter({hasText:'markdown'})).toBeVisible();
});
test('cancel stops the stream and ignores late chunks',async({page})=>{
 await page.goto('/');await page.getByRole('button',{name:'Wait mode'}).click();await page.getByRole('textbox',{name:'Message',exact:true}).fill('hello');await page.getByRole('button',{name:'Send',exact:true}).click();
 await expect(page.getByRole('button',{name:'Stop',exact:true})).toBeVisible();await page.getByRole('button',{name:'Stop',exact:true}).click();
 await expect.poll(()=>page.evaluate(()=>(window as any).chatFixture.aborted)).toBe(1);
 await expect.poll(()=>page.evaluate(()=>(window as any).chatFixture.stopped)).toBe(1);
 await expect(page.getByText('LATE DETACHED CONTENT')).toHaveCount(0);
});
test('leaving disposes the streaming owner',async({page})=>{
 await page.goto('/');await page.getByRole('button',{name:'Wait mode'}).click();await page.getByRole('textbox',{name:'Message',exact:true}).fill('hello');await page.getByRole('button',{name:'Send',exact:true}).click();
 await expect(page.getByRole('button',{name:'Stop',exact:true})).toBeVisible();await page.getByRole('button',{name:'Leave chat'}).click();
 await expect(page.getByText('Left conversation')).toBeVisible();await expect.poll(()=>page.evaluate(()=>(window as any).chatFixture.aborted)).toBe(1);await expect(page.getByText('LATE DETACHED CONTENT')).toHaveCount(0);
});
test('error can retry from the composer',async({page})=>{
 await page.goto('/');await page.getByRole('button',{name:'Error mode'}).click();await page.getByRole('textbox',{name:'Message',exact:true}).fill('hello');await page.getByRole('button',{name:'Send',exact:true}).click();
 await expect(page.getByText('fixture unavailable')).toBeVisible();
 await page.getByRole('textbox',{name:'Message',exact:true}).fill('retry');await page.getByRole('button',{name:'Send',exact:true}).click();await expect(page.locator('strong').filter({hasText:'markdown'})).toBeVisible();
});

// Falsely green if the fixture loads without the consumer stylesheet: verify computed layout and placeholder paint, not class strings.
test('public chat stylesheet renders the composer', async ({ page }) => {
 await page.goto('/');
 const input = page.getByRole('textbox', { name: 'Message', exact: true });
 await expect(input).toHaveCSS('font-size', '14px');
 await expect(input.locator('..')).toHaveCSS('display', 'flex');
 await expect(page.getByRole('button', { name: 'Send', exact: true })).toHaveCSS('border-radius', '8px');
 await expect.poll(() => input.evaluate(element => getComputedStyle(element, '::placeholder').color)).toBe('rgb(156, 163, 175)');
});
