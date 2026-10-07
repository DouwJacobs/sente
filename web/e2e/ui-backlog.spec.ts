import {test,expect,type Page} from '@playwright/test'
async function login(page:Page){await page.goto('/');await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password');await page.getByRole('button',{name:'Sign in',exact:true}).click();await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()}
async function navigate(page:Page,name:string,mobile:boolean){if(mobile&&!['Dashboard','Transactions','Accounts'].includes(name)){await page.getByRole('navigation',{name:'Mobile navigation'}).getByRole('button',{name:'More',exact:true}).click();await page.getByRole('navigation',{name:'More pages'}).getByRole('button',{name,exact:true}).click()}else await page.getByRole('navigation',{name:mobile?'Mobile navigation':'Main navigation',exact:true}).getByRole('button',{name,exact:true}).click()}
for(const width of [1440,360])test(`shared account actions, refresh scope and pending states at ${width}px`,async({page})=>{
 await page.setViewportSize({width,height:900});const mobile=width===360
 const rows:any[]=[{id:1,name:'Synthetic everyday',bank_id:'12345678901',household:1,role:'editor',sync_hidden:0,fnb_connected:1,visibility_allowed:1,version:1,balance_cents:12500,balance_date:'2026-10-03'},{id:2,name:'Synthetic savings',bank_id:'22222222222',household:1,role:'editor',sync_hidden:0,fnb_connected:1,visibility_allowed:1,version:1,balance_cents:25000,balance_date:'2026-10-03'}]
 rows.push({id:3,name:'Synthetic manual account',bank_id:'33333333333',household:1,role:'editor',sync_hidden:0,fnb_connected:0,visibility_allowed:1,version:1,balance_cents:null,balance_date:null})
 let state='ready',scope=0,release:(()=>void)|undefined,fail=false;const requested:number[]=[]
 await page.route('**/api/accounts**',async route=>{
  const req=route.request(),url=new URL(req.url()),body=req.method()==='GET'?null:req.postDataJSON()
  if(req.method()!=='GET'){const row=rows.find(a=>String(a.id)===url.pathname.split('/')[3]);expect(body.version).toBe(row.version);if(url.pathname.endsWith('/visibility'))row.sync_hidden=body.hidden?1:0;else Object.assign(row,body);row.version++;await route.fulfill({json:{ok:true}});return}
  let items=rows.filter(a=>url.pathname.endsWith('/manage')?url.searchParams.has('hidden')?String(a.sync_hidden)===url.searchParams.get('hidden'):true:!a.sync_hidden)
  if(url.searchParams.has('id'))items=items.filter(a=>String(a.id)===url.searchParams.get('id'))
  await route.fulfill({json:{items,total:items.length,list_version:'synthetic-'+rows.map(a=>a.version).join('-')}})
 })
 await page.route('**/api/fnb**',async route=>{
  if(route.request().method()==='GET'){await route.fulfill({json:{connection:{state,version:1,interval_hours:0},refreshing_account_id:scope,refresh_kind:'accounts',accounts:[]}});return}
  const body=route.request().postDataJSON();requested.push(body.account_id||0);await new Promise<void>(resolve=>{release=resolve});await route.fulfill(fail?{status:502,json:{error:'Synthetic refresh failed'}}:{json:{ok:true}})
 })
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message))
 await login(page);await navigate(page,'Accounts',mobile)
 const unmapped=page.locator('.account-summary').filter({hasText:'Synthetic manual account'}),first=page.locator('.account-summary').filter({hasText:'Synthetic everyday'}),second=page.locator('.account-summary').filter({hasText:'Synthetic savings'}),all=page.getByRole('button',{name:'Refresh balances',exact:true})
 await expect(all).toBeEnabled()
 const trigger=first.locator('summary'),icon=trigger.locator('svg'),triggerBox=(await trigger.boundingBox())!,iconBox=(await icon.boundingBox())!
 expect(Math.abs(triggerBox.x+triggerBox.width/2-iconBox.x-iconBox.width/2)).toBeLessThan(1)
 expect(Math.abs(triggerBox.y+triggerBox.height/2-iconBox.y-iconBox.height/2)).toBeLessThan(1)
 await trigger.click();await expect(first.locator('details')).toHaveAttribute('open','')
 await page.getByText(/Household accounts contribute to the shared budget/).click();await expect(first.locator('details')).not.toHaveAttribute('open','')
 await trigger.click();await second.locator('summary').click();await expect(first.locator('details')).not.toHaveAttribute('open','');await expect(second.locator('details')).toHaveAttribute('open','')
 await page.keyboard.press('Escape');await expect(second.locator('details')).not.toHaveAttribute('open','');await expect(second.locator('summary')).toBeFocused()
 await first.locator('summary').focus();await page.keyboard.press('Enter');await first.getByRole('button',{name:'Refresh balance',exact:true}).click()
 await expect(first.getByText('Updating balance',{exact:true})).toBeVisible();await expect(second).toHaveAttribute('aria-busy','false');await expect(all).toBeDisabled();await expect(all).toHaveAttribute('aria-busy','true');expect(await all.evaluate(button=>{const spinner=button.querySelector('.spinner')!.getBoundingClientRect();const text=Array.from(button.childNodes).find(node=>node.nodeType===Node.TEXT_NODE)!;const range=document.createRange();range.selectNode(text);return range.getBoundingClientRect().left-spinner.right})).toBeGreaterThan(4);await expect.poll(()=>requested).toEqual([1])
 release!();await expect(all).toBeEnabled();await expect(first.getByText('Updating balance',{exact:true})).toHaveCount(0)
 fail=true;await all.click();await expect(first).toHaveAttribute('aria-busy','true');await expect(second).toHaveAttribute('aria-busy','true');await expect(unmapped).toHaveAttribute('aria-busy','false');await expect.poll(()=>requested).toEqual([1,0]);release!();await expect(page.locator('.toast:popover-open')).toContainText('Synthetic refresh failed');await expect(all).toBeEnabled();expect(requested).toEqual([1,0])
 // Scheduled status is visible even with the account menu closed.
 await first.locator('summary').focus();await page.keyboard.press('Escape');state='refreshing';scope=1;await expect(first.getByText('Updating balance',{exact:true})).toBeVisible();await expect(second).toHaveAttribute('aria-busy','false');state='ready';await expect(all).toBeEnabled()
 await first.locator('summary').click();await first.getByRole('button',{name:'Edit account',exact:true}).click();const dialog=page.getByRole('dialog',{name:'Edit account',exact:true});await dialog.getByLabel('Account name').fill('Synthetic edited account');await dialog.getByRole('button',{name:'Save account',exact:true}).click();await expect(dialog).not.toBeVisible();await expect(page.getByRole('heading',{name:'Synthetic edited account',exact:true})).toBeVisible()
 const managed=page.locator('.account-summary').filter({hasText:'Synthetic edited account'});await managed.locator('summary').click();await managed.getByRole('button',{name:'Hide account',exact:true}).click();await expect(managed).toHaveCount(0);await page.getByRole('checkbox',{name:/Show hidden accounts/}).check();const hidden=page.locator('.account-settings .line').filter({hasText:'Synthetic edited account'});await hidden.locator('summary').click();await hidden.getByRole('button',{name:'Show account',exact:true}).click();await expect(page.getByRole('heading',{name:'Hidden accounts'})).toBeVisible();await expect(page.locator('.account-summary').filter({hasText:'Synthetic edited account'})).toBeVisible()
 await navigate(page,'Settings',mobile);await page.getByRole('tab',{name:'Banking',exact:true}).click();await expect(page.locator('summary').filter({hasText:'Account visibility'})).toHaveCount(0)
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);expect(errors).toEqual([])
})
for(const width of [1440,360])test(`rule pages retain grouped edit actions and recover from failed loads at ${width}px`,async({page})=>{
 await page.setViewportSize({width,height:900});const mobile=width===360
 const groups=Array.from({length:41},(_,index)=>({index,pattern:'Synthetic paged rule '+String(index).padStart(2,'0'),enabled:1,version:1}));let failNext=true,pauseIds:number[]=[]
 await page.route('**/api/rules**',async route=>{
  const req=route.request(),url=new URL(req.url())
  if(req.method()==='GET'){
   const pageIndex=Number(url.searchParams.get('page')||0)
   if(pageIndex===1&&failNext){failNext=false;await route.fulfill({status:503,json:{error:'Synthetic page unavailable'}});return}
   const items=groups.slice(pageIndex*20,(pageIndex+1)*20).flatMap(g=>[1,2].map(account=>({id:10000+g.index*2+account,account_id:account,account_name:account===1?'Everyday account':'Savings',pattern:g.pattern,category_id:1,category_name:'Groceries',direction:'any',priority:0,enabled:g.enabled,version:g.version,builtin:0})))
   await route.fulfill({json:{items,total:groups.length,list_version:'synthetic-'+groups.map(g=>g.version).join('-')}});return
  }
  const body=req.postDataJSON();pauseIds=body.rules.map((r:any)=>r.id);for(const item of body.rules){const g=groups[Math.floor((item.id-10001)/2)];g.enabled=item.rule.enabled?1:0;g.version++}
  await route.fulfill({json:{ok:true}})
 })
 await login(page);await navigate(page,'Categories',mobile);await page.getByRole('tab',{name:'Automatic rules',exact:true}).click();const panel=page.getByRole('tabpanel',{name:'Automatic rules',exact:true});await expect(panel.locator('.rule-row')).toHaveCount(20);await panel.getByRole('button',{name:'Next',exact:true}).click();await expect(page.locator('.toast:popover-open')).toContainText('Synthetic page unavailable');await expect(panel.locator('.loading-status')).toHaveCount(0);await panel.getByRole('button',{name:'Retry',exact:true}).click();await expect(panel.getByRole('navigation',{name:'List pages'}).getByRole('status')).toContainText('Page 2 of 3')
 const row=panel.locator('.rule-row').filter({hasText:'Synthetic paged rule 20'});await row.getByRole('button',{name:'Pause',exact:true}).click();await expect(row.getByText('Paused',{exact:true})).toBeVisible();expect(pauseIds).toEqual([10041,10042]);await panel.getByRole('button',{name:'Previous',exact:true}).click();await expect(panel.getByRole('navigation',{name:'List pages'}).getByRole('status')).toContainText('Page 1 of 3');await panel.getByRole('button',{name:'Next',exact:true}).click();await expect(row.getByText('Paused',{exact:true})).toBeVisible();expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
})
