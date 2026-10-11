import { test, expect, type Page } from '@playwright/test';
import { start, navigate, contained, moneyFits, noOverflow, reachable } from './responsive-helpers';

import { groupName, categoryName, agentName, accountName, amount, mockResponsiveBudget, mockResponsiveSettings } from './responsive-fixtures';
const pageErrors=new WeakMap<Page,string[]>();
test.beforeEach(({page})=>{const errors:string[]=[];pageErrors.set(page,errors);page.on('pageerror',error=>errors.push(error.message));});
test.afterEach(({page})=>expect(pageErrors.get(page),'no browser runtime errors').toEqual([]));

// Interaction invariants do not need the complete presentation cross product.
const workflowCases = [
 {viewport:{width:360,height:800},theme:'light'},
 {viewport:{width:640,height:360},theme:'dark'},
 {viewport:{width:1440,height:900},theme:'light'},
];
for(const {viewport,theme} of workflowCases) {
 test(`dashboard and budget states ${viewport.width}x${viewport.height} ${theme}`,{tag:'@mobile-smoke'},async({page})=>{
  const {setEmpty}=await mockResponsiveBudget(page);
  await start(page,viewport,theme,'large signed totals, same category in two groups, no-target group');
  await expect(page.locator('.dashboard-overview .budget')).toContainText('Budget remaining');
  for(const value of await page.locator('.dashboard-overview .stat>strong').all()) await moneyFits(page,value,'overview amount');
  const groups=page.locator('.spending-bucket:not(.income-bucket)');
  await expect(groups).toHaveCount(3);
  for(const group of await groups.all()) {
   await group.locator(':scope>summary').click();
   expect(await group.evaluate(e=>getComputedStyle(e).borderBottomWidth),'no trailing group separator').toBe('0px');
  }
  await page.locator('.income-bucket>summary').click();
  for(const value of await page.locator('.budget-figure strong:visible').all()) await moneyFits(page,value,'group/category figure');
  for(const figure of await page.locator('.budget-figure:visible').all()) {
   const label=(await figure.locator('small').boundingBox())!,value=(await figure.locator('strong').boundingBox())!;
   expect(label.x+label.width<=value.x+1||label.y+label.height<=value.y+1,'label/value do not overlap').toBe(true);
  }
  await expect(groups.first()).toContainText('Over budget');
  await expect(groups.nth(2)).toContainText('No budget set');
  await expect(page.locator('.income-bucket')).toContainText('Transfers excluded');
  const spending=(await page.locator('.spending-panel').boundingBox())!,support=(await page.locator('.dashboard-support').boundingBox())!;
  expect(Math.abs(spending.x-support.x),'aligned support left edge').toBeLessThan(2);
  expect(Math.abs(spending.width-support.width),'aligned support width').toBeLessThan(2);
  await groups.first().getByRole('button',{name:categoryName+' transactions',exact:true}).click();
  const detail=page.getByRole('dialog');
  for(const value of await detail.locator('.budget-figure strong').all()) await moneyFits(page,value,'category modal amount');
  await detail.getByRole('button',{name:'Close',exact:true}).click();
  await page.getByLabel("Spending actions", { exact: true }).click(); await page.getByRole('button',{name:'Edit budgets',exact:true}).click();
  const section=page.getByRole('region',{name:groupName+' budget',exact:true});
  await section.locator('.budget-group-summary').click();
  await moneyFits(page,section.locator('.budget-group-amount'),'group total');
  await contained(page,section.getByRole('button',{name:'Add category to '+groupName,exact:true}),'add category',true);
  const pencil=section.getByRole('button',{name:'Edit budget for '+categoryName+' in '+groupName,exact:true});
  await contained(page,pencil,'edit budget category',true);await pencil.click();
  const builder=page.getByRole('dialog');
  const field=builder.getByLabel('Budget amount',{exact:true});
  await expect(field).toHaveValue('4500.00');
  await field.fill('-1');await field.blur();await expect(field).toHaveAttribute('aria-invalid','true');
  await field.fill('987654321.00');await expect(field).not.toHaveAttribute('aria-invalid','true');
  const save=builder.getByRole('button',{name:'Save changes',exact:true});
  await reachable(page,save,'save budget');await contained(page,save,'save budget',true);
  await page.keyboard.press('Escape');
  await section.getByRole('button',{name:'Add category to '+groupName,exact:true}).click();
  const nested=page.getByRole('dialog');
  await expect(nested).toHaveAttribute('aria-label','Add category · '+groupName);
  await nested.getByRole('button',{name:'Save changes',exact:true}).click();
  await expect(nested.getByLabel('Category',{exact:true})).toHaveAttribute('aria-invalid','true');
  await page.keyboard.press('Escape');
  await page.locator('.budget-period-head .toolbar-actions>details>summary').first().click();
  await page.getByRole('button',{name:'View spending',exact:true}).click();
  await page.getByLabel('Accounts',{exact:true}).selectOption('3');
  await expect(page.getByRole('button',{name:'Edit budgets',exact:true})).toHaveCount(0);
  await groups.first().locator(':scope>summary').click();
  await expect(groups.first().getByText('Budget',{exact:true})).toHaveCount(0);
  await expect(page.locator('.dashboard-overview .budget')).toContainText('Net movement');
  setEmpty();await page.getByLabel('Sort budgets',{exact:true}).selectOption('spending');
  await expect(page.getByText('No spending yet',{exact:true})).toBeVisible();
  await noOverflow(page);
 });

 test(`Settings discovery, drafts and management ${viewport.width}x${viewport.height} ${theme}`,{tag:'@mobile-smoke'},async({page})=>{
  await mockResponsiveSettings(page);
  await start(page,viewport,theme,'long user/agent/account/URL, pending proposal; no bank session');
  await navigate(page,'Settings');
  const tabs=page.getByRole('tablist',{name:'Settings sections'});
  // Required sections remain discoverable; new authorized sections may be added.
  for(const name of ['General','Branding','Configuration','Banking','Accounts','Users & access','Backups','Network','PWA','MCP','Notifications','Security','About']) {
   await expect(tabs.getByRole('tab',{name,exact:true})).toBeVisible();
  }
  const general=page.getByRole('tab',{name:'General',exact:true});
  await general.focus();await page.keyboard.press('End');
  const about=page.getByRole('tab',{name:'About',exact:true});
  await expect(about).toBeFocused();await expect(about).toHaveAttribute('aria-selected','true');
  await expect.poll(async()=>{
   const strip=(await tabs.boundingBox())!,active=(await about.boundingBox())!;
   return active.x>=strip.x-1&&active.x+active.width<=strip.x+strip.width+1;
  }).toBe(true);
  await page.keyboard.press('Home');await expect(general).toBeFocused();
  if(viewport.width<=760) {
   const right=page.getByRole('button',{name:'Scroll Settings sections right',exact:true});
   await contained(page,right,'tab scroll',true);
   await right.click();expect(await tabs.evaluate(e=>e.scrollLeft)).toBeGreaterThan(0);
  }
  await page.getByRole('tab',{name:'Branding',exact:true}).click();await page.getByLabel('Display name',{exact:true}).fill('Unsaved synthetic workspace');
  // Every authorized section can be selected; its matching panel stays identified.
  for(const tab of await tabs.getByRole('tab').all()) {
   await tab.click();await expect(tab).toHaveAttribute('aria-selected','true');
   await noOverflow(page);
  }
  await page.getByRole('tab',{name:'Branding',exact:true}).click();await expect(page.getByLabel('Display name',{exact:true})).toHaveValue('Unsaved synthetic workspace');
  await page.getByRole('tab',{name:'Configuration',exact:true}).click();
  const url=page.getByLabel('Repository URL',{exact:true});
  await url.fill('invalid synthetic repository');await url.blur();
  await expect(url).toHaveAttribute('aria-invalid','true');
  await url.fill('https://example.com/'+'long-path'.repeat(25));
  await page.getByLabel('Source name',{exact:true}).fill('Unsaved source');
  await general.click();await page.getByRole('tab',{name:'Configuration',exact:true}).click();
  await expect(page.getByLabel('Source name',{exact:true})).toHaveValue('Unsaved source');
  await expect(url).not.toHaveAttribute('aria-invalid','true');
  await page.route('**/api/configuration/preview',route=>route.fulfill({json:{
   id:'synthetic-responsive-preview',source:{name:agentName},changes:[{entity:'categories',name:agentName,action:'update',
   before:{name:'before'.repeat(50)},after:{name:'after'.repeat(50)}}],
  }}));
  await page.getByRole('button',{name:'Preview bundled starter',exact:true}).click();
  const preview=page.getByRole('region',{name:'Configuration import preview'});
  await preview.locator('summary').click();
  await contained(page,preview,'configuration comparison');
  await preview.getByRole('button',{name:'Import configuration',exact:true}).click();
  const replacement=preview.getByRole('checkbox');
  await expect(replacement).toHaveAttribute('aria-invalid','true');
  await replacement.check();await expect(replacement).not.toHaveAttribute('aria-invalid','true');
  await replacement.uncheck();await expect(replacement).toHaveAttribute('aria-invalid','true');
  await general.click();await page.getByRole('tab',{name:'Configuration',exact:true}).click();
  await expect(preview).toBeVisible();
  await preview.getByRole('button',{name:'Cancel preview',exact:true}).click();
  await page.getByRole('tab',{name:'Banking',exact:true}).click();
  await page.getByLabel('FNB username',{exact:true}).fill('synthetic credential draft');
  await page.getByLabel('FNB password',{exact:true}).fill('synthetic secret draft');
  await general.click();await page.getByRole('tab',{name:'Banking',exact:true}).click();
  await expect(page.getByLabel('FNB username',{exact:true})).toHaveValue('');
  await expect(page.getByLabel('FNB password',{exact:true})).toHaveValue('');
  await page.getByRole('tab',{name:'Users & access',exact:true}).click();
  const row=page.locator('.user-row').first(),menu=row.locator('summary');
  await menu.click();await contained(page,menu,'user menu trigger',true);
  await contained(page,row.locator('.account-action-menu'),'user menu bounds');
  await page.keyboard.press('Escape');await expect(menu).toBeFocused();
  await page.getByRole('tab',{name:'MCP',exact:true}).click();
  const context=page.getByLabel('Context for your agents',{exact:true});
  await context.fill('Unsaved synthetic agent context');
  await general.click();await page.getByRole('tab',{name:'MCP',exact:true}).click();
  await expect(context).toHaveValue('Unsaved synthetic agent context');
  await page.getByText('Inspect exact changes (amounts in cents)',{exact:true}).click();
  for(const code of await page.locator('.mcp-code:visible').all()) await contained(page,code,'proposal JSON');
  const edit=page.getByRole('button',{name:'Edit permissions',exact:true});
  await edit.click();const dialog=page.getByRole('dialog');
  await dialog.getByRole('button',{name:'Save permissions',exact:true}).click();
  await expect(dialog.getByLabel('Confirm permissions',{exact:true})).toHaveAttribute('aria-invalid','true');
  await expect(dialog.getByLabel('Confirm permissions',{exact:true})).toBeFocused();
  for(const check of await dialog.locator('.check:visible').all()) await contained(page,check,'permission choice',viewport.width<=760);
  await dialog.getByLabel('Read your saved MCP context',{exact:true}).check();
  await dialog.getByLabel('Confirm permissions',{exact:true}).selectOption('confirmed');
  await dialog.getByLabel('Read your saved MCP context',{exact:true}).uncheck();
  await expect(dialog.getByLabel('Confirm permissions',{exact:true})).toHaveValue('');
  await reachable(page,dialog.getByRole('button',{name:'Cancel',exact:true}),'permission cancel');
  await page.keyboard.press('Escape');await expect(edit).toBeFocused();
  await navigate(page,'Accounts');
  const account=page.locator('.account-summary').first();
  await expect(account).toContainText(accountName);
  await contained(page,account,'long account row');
  const actions=account.locator('.account-actions>summary');
  await actions.click();await contained(page,actions,'account menu trigger',true);
  await contained(page,account.locator('.account-action-menu'),'account menu bounds');
  await page.keyboard.press('Escape');await expect(actions).toBeFocused();
  await noOverflow(page);
 });
}

for(const width of [599,600,601,759,760,761,899,900,901,1024]) test(`responsive breakpoint ${width}`,async({page})=>{
 await start(page,{width,height:720},'light','Settings and budget breakpoint');
 await navigate(page,'Settings');
 const tabs=page.getByRole('tablist',{name:'Settings sections'});
 await page.getByRole('tab',{name:'General',exact:true}).focus();await page.keyboard.press('End');
 const about=page.getByRole('tab',{name:'About',exact:true});await expect(about).toBeFocused();
 await expect.poll(async()=>{const a=(await about.boundingBox())!,b=(await tabs.boundingBox())!;return a.x>=b.x-1&&a.x+a.width<=b.x+b.width+1}).toBe(true);
 await noOverflow(page);
 await navigate(page,'Budgets');
 const group=page.getByRole('region',{name:'No spending group budget',exact:true});
 await group.locator('.budget-group-summary').click();
 await group.getByRole('button',{name:'Edit budget for Groceries in No spending group',exact:true}).click();
 const dialog=page.getByRole('dialog',{name:'Edit budget · Groceries',exact:true});
 await expect(dialog.getByLabel('Budget amount',{exact:true})).toBeEnabled();
 await contained(page,dialog.getByLabel('Budget amount',{exact:true}),'budget amount field');
 await reachable(page,dialog.getByRole('button',{name:'Save changes',exact:true}),'budget save at breakpoint');
 await noOverflow(page);
});

// Ordinary-user visibility/request coverage lives in settings.spec.ts.

for(const width of [360,1440])test(`responsive loading and errors ${width}`,{tag:width===360?'@mobile-smoke':[]},async({page})=>{
 let release:()=>void=()=>{};
 const gate=new Promise<void>(resolve=>{release=resolve});
 let fail=false;
 await page.route('**/api/dashboard?**',async route=>{
  if(fail)return route.fulfill({status:503,json:{error:'Synthetic unavailable service with a long explanatory message '+ 'detail '.repeat(30)}});
  await gate;await route.continue();
 });
 await start(page,{width,height:800},'dark','delayed dashboard then request-wide error');
 await expect(page.getByText('Loading this period',{exact:true})).toBeVisible();
 await noOverflow(page);release();
 await expect(page.getByLabel('Sort budgets',{exact:true})).toBeVisible();
 fail=true;await page.getByLabel('Sort budgets',{exact:true}).selectOption('spending');
 await expect(page.getByText('Dashboard unavailable',{exact:true})).toBeVisible();
 const toast=page.locator('.toast:popover-open');
 await expect(toast).toContainText('Synthetic unavailable service');
 await contained(page,toast,'request failure toast');
 await noOverflow(page);
});

for(const {width,theme} of [{width:360,theme:'dark'},{width:1440,theme:'light'}])test(`empty builder and budget list ${width} ${theme}`,async({page})=>{
 await page.route('**/api/periods?**',async route=>{
  const response=await route.fetch(),body=await response.json();
  await route.fulfill({json:{...body,items:body.items.map((p:any)=>({...p,name:groupName+' period',target_total:amount,
   group_budgets:[{id:1,name:groupName,target_cents:amount}]}))}});
 });
 let emptyGroups=true;
 await page.route('**/api/periods/1/budget-groups?**',route=>emptyGroups?route.fulfill({json:{items:[],total:0,page:0,page_size:20}}):route.continue());
 await page.route('**/api/periods/1/targets?**',route=>route.fulfill({json:{items:[],total:0,page:0,page_size:20}}));
 await start(page,{width,height:800},theme,'long budget list names/amounts and empty builder');
 await navigate(page,'Budgets');
 for(const value of await page.locator('.budget-periods-panel .budget-total strong,.budget-periods-panel .line strong').all())await moneyFits(page,value,'budget list amount');
 await noOverflow(page);
 const builder=page.locator('.budget-groups-view');
 await expect(builder.getByText('Start building your budget',{exact:true})).toBeVisible();
 await builder.getByRole('button',{name:'Add group',exact:true}).click();
 const picker=page.getByRole('dialog').last();
 await picker.getByLabel('Spending group',{exact:true}).selectOption('1');
 emptyGroups=false;
 await picker.getByRole('button',{name:'Add group to budget',exact:true}).click();
 await expect(picker).toHaveCount(0);
 await builder.getByRole('region',{name:'Day-to-day budget',exact:true}).locator('.budget-group-summary').click();
 await expect(builder.getByText('Add the categories you want to budget for in this group.',{exact:true})).toBeVisible();
 await contained(page,builder.getByRole('button',{name:'Add category to Day-to-day',exact:true}),'empty group add',true);
 await noOverflow(page);
});
