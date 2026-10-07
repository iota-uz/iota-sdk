import {expect,test} from '@playwright/test';

// Falsely green if text alone renders: cross real navigation, built search, browser Mermaid and persisted theme boundaries.
test('static documentation preserves navigation, search, diagrams and theme',async({page})=>{
 const errors:string[]=[];page.on('pageerror',error=>errors.push(error.message));
 await page.goto('/iota-sdk/');await expect(page.locator('main h1')).toHaveText('IOTA SDK Documentation');
 await page.locator('#docs-nav a').filter({hasText:'Architecture'}).first().click();
 await expect(page).toHaveURL(/\/iota-sdk\/architecture(?:\.html)?$/);
 await expect(page.locator('main h1')).toHaveText(/Architecture/);
 await expect(page.locator('.mermaid svg').first()).toBeVisible({timeout:30000});
 await page.getByRole('button',{name:'Toggle theme'}).click();await expect(page.locator('html')).toHaveClass(/dark/);
 await page.reload();await expect(page.locator('html')).toHaveClass(/dark/);
 await page.getByRole('searchbox').fill('tenant');
 const results=page.locator('#search-results a');await expect(results.first()).toBeVisible();
 expect(await results.first().getAttribute('href')).toContain('/iota-sdk/');
 await results.first().click();await expect(page.locator('main h1')).toBeVisible();
 expect(errors).toEqual([]);
});

test('narrow documentation navigation remains keyboard reachable',async({page})=>{
 await page.setViewportSize({width:393,height:852});await page.goto('/iota-sdk/');
 const toggle=page.getByRole('button',{name:'Toggle navigation'});await toggle.focus();await page.keyboard.press('Enter');
 await expect(toggle).toHaveAttribute('aria-expanded','true');await expect(page.locator('#docs-nav')).toBeVisible();
 await page.locator('#docs-nav a').filter({hasText:'Architecture'}).first().click();
 await expect(page.locator('main h1')).toHaveText(/Architecture/);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});
