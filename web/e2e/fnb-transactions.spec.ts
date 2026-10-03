import {test,expect} from '@playwright/test'
for(const width of [1440,360])test(`live FNB preview requires duplicate decisions and confirmation at ${width}px`,async({page})=>{
 await page.setViewportSize({width,height:900})
 let fetched=0,committed=0
 const preview:any={total:2,error_count:0,candidate_count:1,page:0,page_size:50,id:9100,name:'FNB live transactions',format:'fnb-live',bank_id:'12345678901',currency:'ZAR',hash:'synthetic-snapshot',classification_version:'synthetic-rules',run_id:'synthetic-run',coverage:{returned_start:'2026-10-01',returned_end:'2026-10-02',returned_rows:150,possible_gap:true,page_limit_reached:true},rows:[{row:1,date:'2026-10-01',amount_cents:-29,description:'Synthetic live Market purchase',category_id:1,suggestion:'Description rule',source_reference:'synthetic-ref'},{row:2,date:'2026-10-02',amount_cents:-29,description:'Synthetic identical purchase',duplicate:'possible',candidates:[{id:99,date:'2026-10-02',amount_cents:-29,description:'Synthetic identical purchase'}]}]}
 await page.route('**/api/fnb/transactions*',async route=>{expect(route.request().method()).toBe('POST');fetched++;await route.fulfill({json:[preview]})})
 await page.route('**/api/imports**',async route=>{
  const req=route.request()
  if(req.method()==='GET'){if(new URL(req.url()).pathname!=='/api/imports'){await route.fulfill({json:preview});return}await route.fulfill({json:{total:fetched?1:0,items:fetched?[{id:preview.id,account_id:1,name:preview.name,status:committed?'committed':'staged',created_at:'2026-10-03 08:00:00',row_count:2,error_count:0}]:[]}});return}
  expect(req.postDataJSON()).toEqual({confirm_valid_rows:false,skip_all_candidates:false,decisions:{'2':'keep'},classification_version:'synthetic-rules'});committed++;await route.fulfill({json:{inserted:2,skipped:0}})
 })
 await page.goto('/')
 await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password');await page.getByRole('button',{name:'Sign in',exact:true}).click()
 await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
 async function imports(){if(width===360){await page.getByRole('navigation',{name:'Mobile navigation'}).getByRole('button',{name:'More',exact:true}).click();await page.getByRole('navigation',{name:'More pages'}).getByRole('button',{name:'Imports',exact:true}).click()}else await page.getByRole('navigation',{name:'Main navigation',exact:true}).getByRole('button',{name:'Imports',exact:true}).click()}
 await imports();await page.getByRole('button',{name:'Fetch FNB transactions',exact:true}).click()
 await expect(page.getByRole('heading',{name:'FNB live transactions',exact:true})).toBeVisible()
 expect(fetched).toBe(1);expect(committed).toBe(0)
 await expect(page.getByText(/The 150-row page limit was reached/)).toBeVisible()
 await expect(page.getByRole('button',{name:'Confirm import',exact:true})).toBeDisabled()
 await page.reload();await imports()
 await page.getByText('FNB live transactions',{exact:true}).click()
 await page.getByRole('button',{name:'Inspect current preview',exact:true}).click()
 await expect(page.getByText('Description rule',{exact:true})).toBeVisible()
 await page.getByLabel('Decision for row 2').selectOption('keep')
 await page.getByRole('button',{name:'Confirm import',exact:true}).click()
 await expect(page.locator('.toast:popover-open')).toContainText('2 transactions imported; 0 skipped. New entries await review.')
 expect(committed).toBe(1);expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
})
test('live fetch failures show a toast without offering a partial import',async({page})=>{
 await page.route('**/api/fnb/transactions*',route=>route.fulfill({status:502,json:{error:'FNB refresh needs attention: TRANSACTION_LAYOUT_CHANGED'}}))
 await page.goto('/');await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password');await page.getByRole('button',{name:'Sign in',exact:true}).click();await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
 await page.getByRole('navigation',{name:'Main navigation',exact:true}).getByRole('button',{name:'Imports',exact:true}).click();await page.getByRole('button',{name:'Fetch FNB transactions',exact:true}).click()
 await expect(page.locator('.toast:popover-open')).toContainText('TRANSACTION_LAYOUT_CHANGED');await expect(page.getByRole('button',{name:'Confirm import',exact:true})).toHaveCount(0)
})

for(const width of [1440,360])test(`fee components and previews for both accounts are clear at ${width}px`,async({page})=>{
 await page.setViewportSize({width,height:900})
 const report=(id:number,bank:string)=>({total:2,error_count:0,candidate_count:0,page:0,page_size:50,id,name:'FNB live transactions',format:'fnb-live',bank_id:bank,currency:'ZAR',classification_version:'synthetic',coverage:{returned_start:'2026-10-03',returned_end:'2026-10-03',returned_rows:1,service_fee_rows:1,possible_gap:true},rows:[{row:1,date:'2026-10-03',amount_cents:-1234,description:'Synthetic purchase',source_bank_row:1,source_component:'transaction'},{row:2,date:'2026-10-03',amount_cents:-5,description:'Service Fees',category_id:1,source_bank_row:1,source_component:'service_fee',source_bank_description:'Synthetic purchase'}]})
 await page.route('**/api/fnb/transactions*',route=>route.fulfill({json:[report(9201,'12345678901'),report(9202,'22222222222')]}))
 await page.goto('/');await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password');await page.getByRole('button',{name:'Sign in',exact:true}).click();await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
 if(width===360){await page.getByRole('navigation',{name:'Mobile navigation'}).getByRole('button',{name:'More',exact:true}).click();await page.getByRole('navigation',{name:'More pages'}).getByRole('button',{name:'Imports',exact:true}).click()}else await page.getByRole('navigation',{name:'Main navigation',exact:true}).getByRole('button',{name:'Imports',exact:true}).click()
 await page.getByRole('button',{name:'Fetch FNB transactions',exact:true}).click()
 await expect(page.locator('section.preview')).toHaveCount(2)
 await expect(page.locator('.toast:popover-open')).toContainText('FNB previews ready for 2 accounts')
 for(const panel of await page.locator('section.preview').all()){
  await expect(panel.getByText('Bank row 1 service fee · Synthetic purchase',{exact:true})).toBeVisible()
  await expect(panel.getByText(/1 bank rows · 1 additional service-fee entries/)).toBeVisible()
  await expect(panel.getByRole('button',{name:'Confirm import',exact:true})).toBeEnabled()
 }
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
})
