import { test, expect } from '@playwright/test';
import { viewports, start, navigate, moneyFits, noOverflow, reachable, contained } from './responsive-helpers';
import { categoryName, groupName, mockResponsiveBudget, mockResponsiveSettings } from './responsive-fixtures';

// Resize one read-only session instead of repeating every mutation/draft workflow.
for (const theme of ['light', 'dark']) {
 test('Long Settings and budget geometry across all viewports in '+theme, async ({page}) => {
  await mockResponsiveBudget(page);
  await mockResponsiveSettings(page);
  await start(page,viewports[0],theme,'long names and large signed money; geometry only');
  for (const viewport of viewports) {
   await page.setViewportSize(viewport);
   await navigate(page,'Settings');
   const tabs=page.getByRole('tablist',{name:'Settings sections'});
   await page.getByRole('tab',{name:'General',exact:true}).focus();
   await page.keyboard.press('End');
   const about=page.getByRole('tab',{name:'About',exact:true});
   await expect(about).toBeFocused();
   await expect.poll(async()=>{
    const active=(await about.boundingBox())!,strip=(await tabs.boundingBox())!;
    return active.x>=strip.x-1&&active.x+active.width<=strip.x+strip.width+1;
   }).toBe(true);
   for (const tab of await tabs.getByRole('tab').all()) {
    await tab.click();
    await expect(tab).toHaveAttribute('aria-selected','true');
    await noOverflow(page);
   }
   await navigate(page,'Budgets');
   await page.getByRole('button',{name:'Edit budgets',exact:true}).click();
   const group=page.getByRole('region',{name:groupName+' budget',exact:true});
   await group.locator('.budget-group-summary').click();
   await contained(page,group.getByRole('button',{name:'Edit budget for '+categoryName+' in '+groupName,exact:true}),'long category edit',true);
   await group.getByRole('button',{name:'Edit budget for '+categoryName+' in '+groupName,exact:true}).click();
   const builder=page.getByRole('dialog');
   await expect(builder.getByLabel('Budget amount',{exact:true})).toBeEnabled();
   await contained(page,builder.getByLabel('Budget amount',{exact:true}),'budget amount field');
   await reachable(page,builder.getByRole('button',{name:'Save changes',exact:true}),'save budget');
   await noOverflow(page);
   await builder.getByRole('button',{name:'Close',exact:true}).click();
   const nav=page.getByRole('navigation',{name:viewport.width<=760?'Mobile navigation':'Main navigation',exact:true});
   await nav.getByRole('button',{name:'Dashboard',exact:true}).click();
   await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible();
   for (const value of await page.locator('.dashboard-overview .stat>strong').all()) {
    await moneyFits(page,value,'overview amount');
   }
   for (const group of await page.locator('.spending-bucket').all()) {
    const summary=group.locator(':scope>summary');
    if (!await group.evaluate(e=>e.hasAttribute('open'))) await summary.click();
   }
   for (const value of await page.locator('.budget-figure strong:visible').all()) {
    await moneyFits(page,value,'group/category amount');
   }
   for (const figure of await page.locator('.budget-figure:visible').all()) {
    const label=(await figure.locator('small').boundingBox())!,value=(await figure.locator('strong').boundingBox())!;
    expect(label.x+label.width<=value.x+1||label.y+label.height<=value.y+1,'label/value do not overlap').toBe(true);
   }
   await noOverflow(page);
  }
 });
}
