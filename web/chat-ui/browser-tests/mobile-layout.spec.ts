import { expect, test } from '@playwright/test';

for (const width of [360, 390, 430]) {
 test(`mobile header, persisted sidebar and artifacts at ${width}px`, async ({ page }) => {
  await page.setViewportSize({width,height:844});
  await page.addInitScript(()=>localStorage.setItem('bichat-sidebar-collapsed','true'));
  await page.goto('/mobile.html');
  const menu = page.getByRole('button',{name:'Open sidebar',exact:true});
  const assertHeaderClear = async () => {
   const button = await menu.boundingBox();
   const title = await page.getByRole('heading',{level:1}).boundingBox();
   expect(title!.x).toBeGreaterThanOrEqual(button!.x+button!.width);
   expect(title!.y+title!.height).toBeLessThanOrEqual(button!.y+button!.height+12);
  };
  await assertHeaderClear();
  await menu.click();
  const drawer = page.getByRole('dialog');
  await expect(drawer.getByRole('button',{name:'Collapse sidebar',exact:true})).toHaveCount(0);
  await expect(drawer.getByRole('button',{name:'Expand sidebar',exact:true})).toHaveCount(0);
  await expect(drawer.locator('aside')).toHaveCSS('width','256px');
  const panelWidth = await drawer.locator('aside').evaluate(el=>el.parentElement!.parentElement!.getBoundingClientRect().width);
  expect(panelWidth).toBe(256);
  await drawer.getByRole('button',{name:'Close sidebar',exact:true}).click();
  await expect(drawer).toHaveCount(0);
  await page.getByRole('button',{name:'Open existing',exact:true}).click();
  await assertHeaderClear();
  await page.getByRole('button',{name:/Artifacts/,exact:false}).click();
  const artifacts = page.getByRole('dialog',{name:'Artifacts',exact:true});
  await expect(artifacts.getByRole('heading',{name:'Artifacts (0)'})).toBeVisible();
  const heading = artifacts.getByRole('heading',{name:'Artifacts (0)'});
  expect(await heading.evaluate(el=>{const r=el.getBoundingClientRect();return el.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2))})).toBe(true);
  await page.keyboard.press('Escape');
  await expect(artifacts).toHaveCount(0);
 });
}

test('desktop header keeps its original padding and sidebar can collapse', async ({page})=>{
 await page.setViewportSize({width:1280,height:900});
 await page.goto('/mobile.html');
 await expect(page.getByRole('button',{name:'Open sidebar',exact:true})).toHaveCount(0);
 await expect(page.locator('.bichat-header')).toHaveCSS('padding-inline-start','16px');
 await page.getByRole('button',{name:'Collapse sidebar',exact:true}).click();
 await expect(page.getByRole('button',{name:'Expand sidebar',exact:true})).toBeVisible();
});
