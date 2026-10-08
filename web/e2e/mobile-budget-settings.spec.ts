import { test, expect, type Page } from '@playwright/test';
import { reviewPermissions } from '../src/MCPPermissions';
import { viewports, start, navigate, contained, moneyFits, noOverflow, reachable } from './responsive-helpers';

const groupName='Synthetic household essentials and exceptionally long spending group';
const categoryName='Synthetic groceries and supplies with an exceptionally long category';
const agentName='SyntheticAgent'+ 'LongName'.repeat(14);
const accountName='SyntheticAccount'+ 'LongName'.repeat(14);
const amount=98765432100;
const pageErrors=new WeakMap<Page,string[]>();
test.beforeEach(({page})=>{const errors:string[]=[];pageErrors.set(page,errors);page.on('pageerror',error=>errors.push(error.message));});
test.afterEach(({page})=>expect(pageErrors.get(page),'no browser runtime errors').toEqual([]));

for(const viewport of viewports) for(const theme of ['light','dark']) {
 test(`dashboard and budget states ${viewport.width}x${viewport.height} ${theme}`,{tag:viewport.width===390&&theme==='light'||viewport.width===640&&theme==='dark'||viewport.width===1440&&theme==='light'?'@mobile-smoke':[]},async({page})=>{
  let empty=false;
  await page.route('**/api/dashboard?**',async route=>{
   const response=await route.fetch(),body=await response.json();
   const privateScope=!!new URL(route.request().url()).searchParams.get('account');
   const category={id:1,kind:'expense',name:categoryName,target_cents:amount,spent_cents:amount+12345,
    total_target_cents:amount*2,total_spent_cents:amount*2+12345,pending_cents:0};
   await route.fulfill({json:{...body,has_targets:!privateScope,budget_cents:amount*2,spent_cents:empty?0:amount*3,
    remaining_cents:-amount,income_cents:empty?0:amount,pending_spend_cents:0,
    group_total:empty?0:3,categories:empty?[]:[category],category_total:empty?0:1,
    income_categories:empty?[]:[{id:4,name:categoryName,income_cents:amount}],income_category_total:empty?0:1,
    spending_groups:empty?[]:[
     {id:1,name:groupName,color:'teal',target_cents:amount,spent_cents:amount+12345,category_total:1,categories:[category]},
     {id:4,name:groupName+' second',color:'rose',target_cents:amount,spent_cents:-amount,category_total:1,categories:[{...category,spent_cents:-amount}]},
     {id:0,name:'No spending group',color:'slate',target_cents:0,spent_cents:12345,category_total:0,categories:[]},
    ]}});
  });
  await page.route('**/api/periods?**',async route=>{
   const response=await route.fetch(),body=await response.json();
   await route.fulfill({json:{...body,items:body.items.map((p:any)=>({...p,name:groupName+' period',target_total:amount}))}});
  });
  await page.route('**/api/periods/1/targets?**',async route=>{
   const response=await route.fetch(),body=await response.json();
   await route.fulfill({json:{...body,items:body.items.map((c:any)=>({...c,name:c.id===1?categoryName:c.name}))}});
  });
  // Response overrides exercise extreme labels, while the builder uses real fixture IDs/amounts.
  await page.route('**/api/periods/1/budget-groups?**',async route=>{
   const response=await route.fetch(),body=await response.json();
   await route.fulfill({json:{...body,items:body.items.map((g:any)=>({...g,name:groupName}))}});
  });
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
  await page.getByRole('button',{name:'Edit budgets',exact:true}).click();
  const builder=page.getByRole('dialog');
  const field=builder.getByLabel(categoryName,{exact:true});
  await expect(field).toHaveValue('4500.00');
  await field.fill('-1');await field.blur();
  await expect(field).toHaveAttribute('aria-invalid','true');
  await field.fill('987654321.00');await expect(field).not.toHaveAttribute('aria-invalid','true');
  const section=builder.locator('.budget-builder-group').first();
  await moneyFits(page,section.locator('.builder-group-total'),'builder total');
  for(const remove of await section.locator('.budget-builder-category>.button').all())await contained(page,remove,'remove budget category',true);
  await contained(page,section.getByRole('button',{name:'Add category to '+groupName,exact:true}),'add category',true);
  await section.getByRole('button',{name:'Add category to '+groupName,exact:true}).click();
  const nested=page.getByRole('dialog').last();
  await expect(nested).toHaveAttribute('aria-label','Add category · '+groupName);
  await nested.getByRole('button',{name:'Add category to budget',exact:true}).click();
  await expect(nested.getByLabel('Category',{exact:true})).toHaveAttribute('aria-invalid','true');
  await page.keyboard.press('Escape');
  await expect(field).toHaveValue('987654321.00');
  const save=builder.getByRole('button',{name:'Save budget',exact:true});
  await reachable(page,save,'save budget');await contained(page,save,'save budget',true);
  await builder.getByRole('button',{name:'Close',exact:true}).click();
  await page.getByLabel('Accounts',{exact:true}).selectOption('3');
  await expect(page.getByRole('button',{name:'Edit budgets',exact:true})).toHaveCount(0);
  await groups.first().locator(':scope>summary').click();
  await expect(groups.first().getByText('Budget',{exact:true})).toHaveCount(0);
  await expect(page.locator('.dashboard-overview .budget')).toContainText('Net movement');
  empty=true;await page.getByLabel('Sort budgets',{exact:true}).selectOption('spending');
  await expect(page.getByText('No spending yet',{exact:true})).toBeVisible();
  await noOverflow(page);
 });

 test(`Settings discovery, drafts and management ${viewport.width}x${viewport.height} ${theme}`,{tag:viewport.width===390&&theme==='light'||viewport.width===640&&theme==='dark'||viewport.width===1440&&theme==='light'?'@mobile-smoke':[]},async({page})=>{
  await page.route('**/api/users?**',async route=>{
   const response=await route.fetch(),body=await response.json();
   await route.fulfill({json:{...body,items:body.items.map((u:any)=>({...u,username:'SyntheticUser'+'LongName'.repeat(12)}))}});
  });
  await page.route('**/api/accounts?**',async route=>{
   const response=await route.fetch(),body=await response.json();
   await route.fulfill({json:{...body,items:body.items.map((a:any)=>({...a,name:accountName,balance_cents:-amount}))}});
  });
  await page.route('**/api/configuration',async route=>{
   const response=await route.fetch(),body=await response.json();
   await route.fulfill({json:{...body,default_source:{...body.default_source,repository_url:'https://example.com/'+'long-path'.repeat(25)},
    sources:[{id:999,name:agentName,kind:'repository',repository_url:'https://example.com/'+'long-path'.repeat(25),ref:'main',path:'sente.json',version:1,last_revision:'long-revision'.repeat(20)}]}});
  });
  await page.route('**/api/fnb',route=>route.fulfill({json:{connection:null,accounts:[]}}));
  await page.route('**/api/mcp/settings',route=>route.fulfill({json:{endpoint:'https://example.com/'+'endpoint'.repeat(30),
   accounts:[{id:1,name:accountName,can_edit:true}],
   connections:[{id:'synthetic-layout-agent',name:agentName,permission_version:1,can_write:true,expires_at:2000000000,
    permissions:{...reviewPermissions(),preset:'custom',capabilities:{assign_missing:true,create_rule:true},constraints:{...reviewPermissions().constraints,account_ids:[1],selected_accounts:true}}}],
   proposals:[{id:'synthetic-layout-proposal',operation:'edit_transactions',token_name:agentName,status:'pending',expires_at:2000000000,
    payload:{change:{description:'updated-description'.repeat(40),amount_cents:-amount},before:{description:'long-description'.repeat(40),amount_cents:-amount},after:{description:'updated-description'.repeat(40),amount_cents:-amount}}}]}}));
  await start(page,viewport,theme,'long user/agent/account/URL, pending proposal; no bank session');
  await navigate(page,'Settings');
  const tabs=page.getByRole('tablist',{name:'Settings sections'});
  await expect(tabs.getByRole('tab')).toHaveText(['General','Branding','Configuration','Banking','Accounts','Users & access','Backups','Network','PWA','MCP','Notifications','Security','About']);
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
 await navigate(page,'Budgets');await page.getByRole('button',{name:'Edit budgets',exact:true}).click();
 const dialog=page.getByRole('dialog');
 await contained(page,dialog.getByLabel('Groceries',{exact:true}),'budget amount field');
 await reachable(page,dialog.getByRole('button',{name:'Save budget',exact:true}),'budget save at breakpoint');
 await noOverflow(page);
});

for(const width of [360,390,430])test(`ordinary Settings sections ${width}`,{tag:width===360?'@mobile-smoke':[]},async({page})=>{
 await page.route('**/api/me',async route=>{
  const response=await route.fetch();if(!response.ok())return route.fulfill({response});
  const body=await response.json();await route.fulfill({json:{...body,user:{...body.user,admin:false}}});
 });
 await start(page,{width,height:800},'dark','ordinary-user section visibility (presentation only)');
 await navigate(page,'Settings');
 await expect(page.getByRole('tablist',{name:'Settings sections'}).getByRole('tab')).toHaveText(['General','PWA','MCP','Notifications','Security','About']);
 for(const name of ['General','PWA','MCP','Notifications','Security','About']) {
  await page.getByRole('tab',{name,exact:true}).click();await noOverflow(page);
 }
});

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

for(const width of [360,1440])for(const theme of ['light','dark'])test(`empty builder and budget list ${width} ${theme}`,async({page})=>{
 await page.route('**/api/periods?**',async route=>{
  const response=await route.fetch(),body=await response.json();
  await route.fulfill({json:{...body,items:body.items.map((p:any)=>({...p,name:groupName+' period',target_total:amount,
   group_budgets:[{id:1,name:groupName,target_cents:amount}]}))}});
 });
 await page.route('**/api/periods/1/budget-groups?**',route=>route.fulfill({json:{items:[],total:0,page:0,page_size:20}}));
 await page.route('**/api/periods/1/targets?**',route=>route.fulfill({json:{items:[],total:0,page:0,page_size:20}}));
 await start(page,{width,height:800},theme,'long budget list names/amounts and empty builder');
 await navigate(page,'Budgets');
 for(const value of await page.locator('.budget-periods-panel .budget-total strong,.budget-periods-panel .line strong').all())await moneyFits(page,value,'budget list amount');
 await noOverflow(page);
 await page.getByRole('button',{name:'Edit budgets',exact:true}).click();
 const builder=page.getByRole('dialog').first();
 await expect(builder.getByText('Start building your budget',{exact:true})).toBeVisible();
 await builder.getByRole('button',{name:'Add group',exact:true}).click();
 const picker=page.getByRole('dialog').last();
 await picker.getByLabel('Spending group',{exact:true}).selectOption('1');
 await picker.getByRole('button',{name:'Add group to budget',exact:true}).click();
 await expect(builder.getByText('Add the categories you want to budget for in this group.',{exact:true})).toBeVisible();
 await reachable(page,builder.getByRole('button',{name:'Save budget',exact:true}),'empty group save');
 await builder.getByRole('button',{name:'Close',exact:true}).click();
 await noOverflow(page);
});
