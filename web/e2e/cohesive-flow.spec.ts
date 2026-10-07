import {test,expect} from '@playwright/test'
for(const width of [1440,360])for(const theme of ['light','dark'])test(`cohesive import and compact screens ${width} ${theme}`,async({page})=>{
 await page.setViewportSize({width,height:900});await page.addInitScript(t=>localStorage.setItem('finance-theme',t),theme)
 let fetched=false,added:number[]=[]
 const preview=(id:number,bank:string)=>({id,name:'FNB live transactions',format:'fnb-live',bank_id:bank,currency:'ZAR',total:1,candidate_count:id===9101?1:0,error_count:0,classification_version:'rules',rows:[{row:1,date:'2026-10-01',description:'Synthetic purchase',amount_cents:-100,category_id:1,...(id===9101?{duplicate:'possible',candidates:[{id:99,date:'2026-10-01',description:'Synthetic purchase',amount_cents:-100}]}:{})}],coverage:{returned_rows:1,possible_gap:true}})
 await page.route('**/api/fnb?summary=1',route=>route.fulfill({json:{connection:{state:'ready'}}}))
 await page.route('**/api/fnb/transactions*',route=>{fetched=true;return route.fulfill({json:[preview(9101,'12345678901'),preview(9102,'22222222222')]})})
 await page.route('**/api/imports**',route=>{
  const url=new URL(route.request().url()),id=Number(url.pathname.split('/')[3])
  if(route.request().method()==='POST'){expect(url.pathname).toContain('/commit');added.push(id);return route.fulfill({json:{inserted:1,skipped:0}})}
  if(id)return route.fulfill({json:preview(id,id===9101?'12345678901':'22222222222')})
  return route.fulfill({json:{total:fetched?1:0,pending_total:fetched?2-added.length:0,items:fetched?[9101,9102].map((id,i)=>({id,account_id:i+1,account_name:i?'Private savings':'Everyday account',account_ending:i?'2222':'8901',run_id:'run-one',format:'fnb-live',name:'FNB live transactions',status:added.includes(id)?'committed':'staged',created_at:'2026-10-03 08:00:00',row_count:1,error_count:0,inserted:1,skipped:0})):[]}})
 })
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message))
 await page.goto('/');await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password');await page.getByRole('button',{name:'Sign in',exact:true}).click();await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
 const nav=page.getByRole('navigation',{name:width===360?'Mobile navigation':'Main navigation',exact:true})
 await nav.getByRole('button',{name:'Transactions',exact:true}).click();await expect(nav.getByRole('button',{name:'Review',exact:true})).toHaveCount(0)
 await page.getByRole('tab',{name:/Import activity/}).click();await page.getByRole('button',{name:'Get transactions',exact:true}).click()
 await expect(page.locator('section.preview')).toHaveCount(1)
 expect(added).toEqual([9102])
 await expect(page.locator('section.preview').first().getByRole('button',{name:'Continue to review'})).toBeDisabled()
 await expect(page.getByRole('button',{name:'Resolve issues below'})).toBeDisabled()
 await page.getByLabel('Decision for row 1').selectOption('keep')
 await expect(page.getByRole('button',{name:'Send 1 account to review'})).toBeEnabled()
 await page.getByRole('button',{name:'Send 1 account to review'}).click()
 await expect(page.getByRole('tab',{name:'All transactions',exact:true})).toHaveAttribute('aria-selected','true');expect(added.sort()).toEqual([9101,9102])
 await expect(page.getByLabel('Budget period',{exact:true})).toHaveValue('');await expect(page.getByLabel('Accounts',{exact:true})).toHaveValue('')
 await page.getByRole('tab',{name:/Import activity/}).click();await page.locator('.activity-run>summary').click();await expect(page.locator('.activity-account')).toHaveCount(2);await expect(page.locator('.activity-account').first()).toContainText('Everyday account')
 await nav.getByRole('button',{name:'Accounts',exact:true}).click();await expect(page.locator('.account-summary').first()).toBeVisible();expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
 if(width===360){await nav.getByRole('button',{name:'More',exact:true}).click();await page.getByRole('navigation',{name:'More pages'}).getByRole('button',{name:'Categories',exact:true}).click()}else await nav.getByRole('button',{name:'Categories',exact:true}).click()
 await expect(page.getByRole('tab',{name:'Categories',exact:true})).toHaveAttribute('aria-selected','true');await page.getByRole('tab',{name:'Automatic rules',exact:true}).click();await expect(page.getByRole('tabpanel',{name:'Automatic rules'})).toBeVisible()
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);expect(errors).toEqual([])
})

test('clean export automatically opens out-of-period transactions for review and ledger',async({page})=>{
 await page.goto('/');await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password');await page.getByRole('button',{name:'Sign in',exact:true}).click();await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
 await page.getByRole('navigation',{name:'Main navigation',exact:true}).getByRole('button',{name:'Transactions',exact:true}).click();await page.getByRole('tab',{name:/Import activity/}).click();await page.getByText('Upload a bank export',{exact:true}).click()
 await page.getByLabel('Import into account',{exact:true}).selectOption('1')
 const description='Market synthetic cohesive outside period'
 const csv='ACCOUNT TRANSACTION HISTORY\n\nName:,Synthetic\nAccount:,12345678901,Fusion\nBalance:,100.00\n\nDate, Amount, Balance, Description\n2026/09/30,-1.01,100.00,'+description+'\n'
 await page.locator('input[type=file]').setInputFiles({name:'synthetic-cohesive.csv',mimeType:'text/csv',buffer:Buffer.from(csv)})
 await page.getByRole('button',{name:'Import statement',exact:true}).click();await expect(page.getByRole('tab',{name:'All transactions',exact:true})).toHaveAttribute('aria-selected','true')
 await expect(page.getByLabel('Budget period',{exact:true})).toHaveValue('')
 const entry=page.locator('.transaction-detail').filter({hasText:description});await expect(entry).toContainText('Unseen');await expect(entry).not.toContainText('Needs category')
 const saved=await page.evaluate(async(description)=>{const rows=await fetch('/api/transactions?query='+encodeURIComponent(description)).then(r=>r.json());return rows.items.find((row:{description:string})=>row.description===description)},description);expect(saved.review_state).toBe('approved');expect(saved.seen).toBeFalsy()
 await page.getByRole('tab',{name:/Needs review/}).click();await expect(entry).toHaveCount(0)

})

for(const width of [1440,360])for(const theme of ['light','dark'])test(`empty state spacing and neutral modal ${width} ${theme}`,async({page})=>{
 await page.setViewportSize({width,height:900});await page.addInitScript(t=>localStorage.setItem('finance-theme',t),theme)
 await page.route('**/api/transactions?**',route=>route.fulfill({json:{items:[],total:0,list_version:'empty'}}))
 await page.route('**/api/imports?**',route=>route.fulfill({json:{items:[],total:0,pending_total:7}}))
 await page.goto('/');await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password');await page.getByRole('button',{name:'Sign in',exact:true}).click();await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
 const nav=page.getByRole('navigation',{name:width===360?'Mobile navigation':'Main navigation',exact:true})
 await nav.getByRole('button',{name:'Transactions',exact:true}).click()
 for(const tab of ['All transactions','Needs review']){
  await page.getByRole('tab',{name:new RegExp(tab)}).click();await expect(page.locator('.empty-description')).toContainText('7 imports need attention.')
  await expect.poll(async()=>page.evaluate(()=>{
   const text=document.querySelector('.empty-description')
   const button=Array.from(document.querySelectorAll('.empty-actions button')).find(b=>b.textContent?.includes('Resolve import issues'))
   if(!text||!button||!text.getClientRects().length||!button.getClientRects().length)return -1
   return button.getBoundingClientRect().y-text.getBoundingClientRect().bottom
  })).toBeGreaterThanOrEqual(15)
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
 }
 await nav.getByRole('button',{name:'Accounts',exact:true}).click();await page.getByLabel('Account management',{exact:true}).click();await page.getByRole('button',{name:'Add account',exact:true}).click();await expect(page.getByRole('dialog')).toBeVisible()
 expect(await page.getByRole('dialog').evaluate(el=>getComputedStyle(el,'::backdrop').backgroundColor)).toBe('rgba(16, 18, 23, 0.48)')
})

test('existing clean staged imports are processed while invalid rows still require a decision',async({page})=>{
 let committed:number[]=[]
 const report=(id:number)=>({id,account_name:'Everyday account',bank_id:'12345678901',format:'fnb-live',currency:'ZAR',total:1,error_count:id===9302?1:0,candidate_count:0,classification_version:'synthetic',rows:[{row:1,date:'2026-10-03',description:'Synthetic staged transaction',amount_cents:-100,...(id===9302?{error:'Invalid date'}:{})}]})
 await page.route('**/api/imports**',route=>{
  const url=new URL(route.request().url()),id=Number(url.pathname.split('/')[3])
  if(route.request().method()==='POST'){committed.push(id);return route.fulfill({json:{inserted:1,skipped:0}})}
  if(id)return route.fulfill({json:report(id)})
  return route.fulfill({json:{items:[9301,9302].map(id=>({id,account_id:1,account_name:'Everyday account',run_id:'staged-run',format:'fnb-live',status:committed.includes(id)?'committed':'staged',created_at:'2026-10-03 08:00:00',row_count:1,error_count:id===9302?1:0})),total:1,pending_total:2-committed.length}})
 })
 await page.goto('/');await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password');await page.getByRole('button',{name:'Sign in',exact:true}).click();await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
 await page.getByRole('navigation',{name:'Main navigation',exact:true}).getByRole('button',{name:'Transactions',exact:true}).click();await page.getByRole('tab',{name:/Import activity/}).click()
 await expect.poll(()=>committed).toEqual([9301]);await expect(page.locator('section.preview')).toHaveCount(1);await expect(page.getByRole('button',{name:'Continue to review',exact:true})).toBeDisabled()
 await page.locator('.activity-run>summary').click();await expect(page.getByRole('navigation',{name:'List pages',exact:true})).toHaveCount(0)
 expect(committed).toEqual([9301])
})
