import {test,expect} from '@playwright/test'
for(const width of [1440,360])test(`live FNB preview requires duplicate decisions and confirmation at ${width}px`,async({page})=>{
 await page.setViewportSize({width,height:900})
 let fetched=0,committed=0
 const preview:any={total:2,error_count:0,candidate_count:1,page:0,page_size:50,id:9100,name:'FNB live transactions',format:'fnb-live',bank_id:'12345678901',currency:'ZAR',hash:'synthetic-snapshot',classification_version:'synthetic-rules',run_id:'synthetic-run',coverage:{returned_start:'2026-10-01',returned_end:'2026-10-02',returned_rows:150,possible_gap:true,page_limit_reached:true},rows:[{row:1,date:'2026-10-01',amount_cents:-29,description:'Synthetic live Market purchase',category_id:1,suggestion:'Description rule',source_reference:'synthetic-ref'},{row:2,date:'2026-10-02',amount_cents:-29,description:'Synthetic identical purchase',duplicate:'possible',candidates:[{id:99,date:'2026-10-02',amount_cents:-29,description:'Synthetic identical purchase'}]}]}
 await page.route('**/api/fnb/transactions*',async route=>{expect(route.request().method()).toBe('POST');fetched++;await route.fulfill({json:[preview]})})
 await page.route('**/api/imports**',async route=>{
  const req=route.request()
  if(req.method()==='GET'){if(new URL(req.url()).pathname!=='/api/imports'){await route.fulfill({json:preview});return}await route.fulfill({json:{total:fetched?1:0,items:fetched?[{id:preview.id,account_id:1,account_name:'Everyday account',account_ending:'8901',format:'fnb-live',run_id:'synthetic-run',name:preview.name,status:committed?'committed':'staged',created_at:'2026-10-03 08:00:00',row_count:2,error_count:0}]:[]}});return}
  expect(req.postDataJSON()).toEqual({confirm_valid_rows:false,skip_all_candidates:false,decisions:{'2':'keep'},classification_version:'synthetic-rules'});committed++;await route.fulfill({json:{inserted:2,skipped:0}})
 })
 await page.route('**/api/fnb?summary=1',route=>route.fulfill({json:{connection:{state:'ready'}}}));await page.goto('/')
 await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password');await page.getByRole('button',{name:'Sign in',exact:true}).click()
 await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
 async function imports(){await page.getByRole('navigation',{name:width===360?'Mobile navigation':'Main navigation',exact:true}).getByRole('button',{name:'Transactions',exact:true}).click();await page.getByRole('tab',{name:/Import activity/}).click()}
 await imports();await page.getByRole('button',{name:'Get transactions',exact:true}).click()
 await expect(page.locator('section.preview')).toHaveCount(1)
 expect(fetched).toBe(1);expect(committed).toBe(0)
 await expect(page.getByText(/FNB returned its maximum of 150 transactions/)).toBeVisible()
 await expect(page.getByRole('button',{name:'Continue to review',exact:true})).toBeDisabled()
 await page.reload();await imports()
 await page.locator('.activity-run>summary').click()
 await page.getByRole('button',{name:'Resolve import issues',exact:true}).click()
 await expect(page.getByText('Description rule',{exact:true})).toBeVisible()
 await page.getByLabel('Decision for row 2').selectOption('keep')
 await page.getByRole('button',{name:'Continue to review',exact:true}).click()
 await expect(page.locator('.toast:popover-open')).toContainText('2 transactions added; 0 skipped. Categorized transactions are accepted and unseen.')
 expect(committed).toBe(1);expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
})
test('live fetch failures show a toast without offering a partial import',async({page})=>{
 await page.route('**/api/fnb/transactions*',route=>route.fulfill({status:502,json:{error:'FNB refresh needs attention: TRANSACTION_LAYOUT_CHANGED'}}))
 await page.route('**/api/fnb?summary=1',route=>route.fulfill({json:{connection:{state:'ready'}}}));await page.goto('/');await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password');await page.getByRole('button',{name:'Sign in',exact:true}).click();await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
 await page.getByRole('navigation',{name:'Main navigation',exact:true}).getByRole('button',{name:'Transactions',exact:true}).click();await page.getByRole('tab',{name:/Import activity/}).click();await page.getByRole('button',{name:'Get transactions',exact:true}).click()
 await expect(page.locator('.toast:popover-open')).toContainText('TRANSACTION_LAYOUT_CHANGED');await expect(page.getByRole('button',{name:'Continue to review',exact:true})).toHaveCount(0)
})

for(const width of [1440,360])test(`clean bank transactions and fees import automatically at ${width}px`,async({page})=>{
 await page.setViewportSize({width,height:900})
 let commits:number[]=[]
 const report=(id:number,bank:string)=>({total:2,error_count:0,candidate_count:0,id,name:'FNB live transactions',format:'fnb-live',bank_id:bank,currency:'ZAR',classification_version:'synthetic',rows:[{row:1,date:'2026-10-03',amount_cents:-1234,description:'Synthetic purchase'},{row:2,date:'2026-10-03',amount_cents:-5,description:'Service Fees',category_id:1,source_component:'service_fee'}]})
 await page.route('**/api/fnb/transactions*',route=>route.fulfill({json:[report(9201,'12345678901'),report(9202,'22222222222')]}))
 await page.route('**/api/imports/*/commit',route=>{const body=route.request().postDataJSON();expect(body.decisions).toEqual({});expect(body.confirm_valid_rows).toBe(false);commits.push(Number(new URL(route.request().url()).pathname.split('/')[3]));return route.fulfill({json:{inserted:2,skipped:0}})})
 await page.route('**/api/transactions**',route=>{const url=new URL(route.request().url());return url.searchParams.get('pending')==='1'&&url.searchParams.has('imports')?route.fulfill({json:{total:1,items:[]}}):route.fallback()})
 await page.route('**/api/fnb?summary=1',route=>route.fulfill({json:{connection:{state:'ready'}}}));await page.goto('/');await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password');await page.getByRole('button',{name:'Sign in',exact:true}).click();await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
 await page.getByRole('navigation',{name:width===360?'Mobile navigation':'Main navigation',exact:true}).getByRole('button',{name:'Transactions',exact:true}).click();await page.getByRole('tab',{name:/Import activity/}).click();await page.getByRole('button',{name:'Get transactions',exact:true}).click()
 await expect(page.getByRole('tab',{name:/Needs review/})).toHaveAttribute('aria-selected','true');expect(commits).toEqual([9201,9202]);await expect(page.locator('section.preview')).toHaveCount(0)
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
})

for(const width of [1440,360])test(`server imports repeated FNB pages without duplicate decisions at ${width}px`,async({page})=>{
 await page.setViewportSize({width,height:900})
 let fetches=0,commits=0
 await page.route('**/api/fnb/transactions*',route=>{fetches++;return route.fulfill({json:[{id:9301,total:3,error_count:0,candidate_count:0,format:'fnb-live',bank_id:'12345678901',currency:'ZAR',already_imported:true,inserted:fetches===1?1:0,skipped:2,rows:[]}]})})
 await page.route('**/api/imports/*/commit',route=>{commits++;return route.fulfill({status:409,json:{error:'This import is already committed'}})})
 await page.route('**/api/transactions**',route=>{const url=new URL(route.request().url());return url.searchParams.get('pending')==='1'&&url.searchParams.has('imports')?route.fulfill({json:{total:width===1440?1:0,items:[]}}):route.fallback()})
 await page.route('**/api/fnb?summary=1',route=>route.fulfill({json:{connection:{state:'ready'}}}))
 await page.goto('/');await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password');await page.getByRole('button',{name:'Sign in',exact:true}).click()
 await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
 await page.getByRole('navigation',{name:width===360?'Mobile navigation':'Main navigation',exact:true}).getByRole('button',{name:'Transactions',exact:true}).click()
 await page.getByRole('tab',{name:/Import activity/}).click();await page.getByRole('button',{name:'Get transactions',exact:true}).click()
 await expect(page.getByRole('tab',{name:width===1440?/Needs review/:/All transactions/})).toHaveAttribute('aria-selected','true')
 await page.getByRole('tab',{name:/Import activity/}).click();await page.getByRole('button',{name:'Get transactions',exact:true}).click()
 await expect.poll(()=>fetches).toBe(2)
 await expect(page.getByRole('button',{name:'Get transactions',exact:true})).toBeEnabled()
 expect(commits).toBe(0)
 await expect(page.locator('section.preview')).toHaveCount(0)
 await expect(page.getByRole('tab',{name:/Import activity/})).toHaveAttribute('aria-selected','true')
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
})
