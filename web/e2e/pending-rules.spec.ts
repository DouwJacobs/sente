import { desktopPhoneCases } from './coverage-cases';
import {test,expect} from '@playwright/test'
for(const {width,theme} of desktopPhoneCases)test(`transaction rule offers missing-only backfill ${width} ${theme}`,async({page})=>{
 await page.setViewportSize({width,height:900})
 await page.addInitScript(t=>localStorage.setItem('finance-theme',t),theme)
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message))
 await page.goto('/')
 await page.getByLabel('Username',{exact:true}).fill('demo')
 await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password')
 await page.getByRole('button',{name:'Sign in',exact:true}).click()
 await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
 await page.getByRole('navigation',{name:width===360?'Mobile navigation':'Main navigation',exact:true}).getByRole('button',{name:'Transactions',exact:true}).click()
 await page.getByRole('tab',{name:/Import activity/}).click()
 await page.getByText('Upload a bank export',{exact:true}).click()
 await page.getByLabel('Import into account',{exact:true}).selectOption('1')
 const tag=`Reclassify ${width===360?'mobile':'desktop'} ${theme}`
 const descriptions=[`${tag} source`,`${tag} pending`,`${tag} manual`,`Different supplier ${width} ${theme}`]
 const csv='ACCOUNT TRANSACTION HISTORY\n\nName:,Synthetic\nAccount:,12345678901,Fusion\nBalance:,100.00\n\nDate, Amount, Balance, Description\n'+descriptions.map((description,i)=>`2026/10/27,-${i+1}.0${i+1},100.00,${description}`).join('\n')+'\n'
 await page.locator('input[type=file]').setInputFiles({name:`pending-${width}-${theme}.csv`,mimeType:'text/csv',buffer:Buffer.from(csv)})
 await page.getByRole('button',{name:'Import statement',exact:true}).click()
 await expect(page.getByRole('tab',{name:/Needs review/})).toHaveAttribute('aria-selected','true')
 const entries=page.locator('.transaction-detail')
 await expect(entries.filter({hasText:descriptions[0]})).toHaveCount(1)
 // Make one matching entry a manual classification through the normal API.
 await page.evaluate(async description=>{
  const session=await fetch('/api/me').then(r=>r.json())
  const items=(await fetch('/api/transactions?q='+encodeURIComponent(description)).then(r=>r.json())).items
  const t=items.find((t:any)=>t.description===description)
  const groups=(await fetch('/api/spending-groups').then(r=>r.json()))
  const response=await fetch('/api/transactions/'+t.id,{method:'PUT',headers:{'Content-Type':'application/json','X-CSRF-Token':session.csrf},body:JSON.stringify({version:t.version,date:t.date,amount_cents:t.amount_cents,description:t.description,allocations:[{category_id:3,amount_cents:t.amount_cents,note:'Manual allocation note'}],spending_group_id:groups.find((g:any)=>g.name==='Exceptions').id,is_transfer:false,assignment:t.assignment,period_id:t.period_id})})
  if(!response.ok)throw new Error('Synthetic fixture setup failed')
 },descriptions[2])
 if(width===360){await entries.filter({hasText:descriptions[1]}).press('Space');await page.getByLabel('Transaction actions',{exact:true}).click()}else await page.getByRole('checkbox',{name:'Select '+descriptions[1],exact:true}).check()
 await expect(page.getByRole('button',{name:'Mark seen (1)',exact:true})).toBeVisible()
 if(width===360)await page.getByRole('button',{name:'Cancel selection',exact:true}).click()
 await entries.filter({hasText:descriptions[0]}).click()
 const editor=page.getByRole('dialog',{name:/^Transaction #[0-9]+$/})
 await editor.getByLabel('Category',{exact:true}).click()
 await page.getByRole('dialog',{name:'Select category',exact:true}).getByRole('button').filter({hasText:'Groceries'}).first().click()
 await editor.getByLabel('Spending group',{exact:true}).click()
 await page.getByRole('dialog',{name:'Select spending group',exact:true}).getByRole('button',{name:/^Recurring(?: Selected)?$/}).click()
 await editor.locator('summary').filter({hasText:'Automatically categorize similar transactions'}).click()
 await expect(editor.getByText(/After saving/)).toBeVisible()
 await editor.getByLabel('Use this category and spending group for similar transactions in this account',{exact:true}).check()
 await editor.getByLabel('Description contains',{exact:true}).fill(tag)
 await editor.getByRole('button',{name:'Save changes',exact:true}).click()
 await expect(editor).not.toBeVisible()
 const offered=page.getByRole('dialog',{name:'Apply this rule to previous transactions?',exact:true});
 await expect(offered).toBeVisible();
 const unchanged=await page.evaluate(async description=>(await fetch('/api/transactions?q='+encodeURIComponent(description)).then(r=>r.json())).items.find((t:any)=>t.description===description),descriptions[1]);
 expect(unchanged.allocations[0].category_id).toBeNull();expect(unchanged.version).toBe(1);
 await offered.getByLabel('Previous transactions',{exact:true}).selectOption('period');await offered.getByLabel('Budget period',{exact:true}).selectOption('1');
 await offered.getByRole('button',{name:'Preview changes',exact:true}).click();
 await expect(offered.getByRole('button',{name:'Apply 1 changes',exact:true})).toBeVisible();
 await offered.getByRole('button',{name:'Apply 1 changes',exact:true}).click();await expect(offered).toBeHidden();
 const saved=await page.evaluate(async descriptions=>{
  const items=(await fetch('/api/transactions').then(r=>r.json())).items
  return descriptions.map(description=>items.find((t:any)=>t.description===description))
 },descriptions)
 expect(saved[0].review_state).toBe('approved')
 expect(saved[1].allocations[0].category_id).toBe(1)
 expect(saved[1].spending_group_name).toBe('Recurring')
 expect(saved[1].review_state).toBe('approved')
 expect(saved[1].seen).toBe(0)
 expect(saved[0].seen).toBe(1)
 expect(saved[1].version).toBe(2)
 expect(saved[2].allocations[0].category_id).toBe(3)
 expect(saved[2].allocations[0].note).toBe('Manual allocation note')
 expect(saved[2].spending_group_name).toBe('Exceptions')
 expect(saved[2].version).toBe(2)
 expect(saved[3].allocations[0].category_id).toBeNull()
 expect(saved[3].version).toBe(1)
 const history=await page.evaluate(async id=>(await fetch('/api/audit/'+id).then(r=>r.json())),saved[1].id)
 expect(history.some((entry:any)=>entry.action==='rule_backfilled')).toBe(true)
 await page.getByRole('button',{name:/^Filters/}).click()
 await page.getByLabel('Filter by category',{exact:true}).selectOption('uncategorized')
 await expect(entries.filter({hasText:descriptions[1]})).toHaveCount(0)
 await expect(entries.filter({hasText:descriptions[3]})).toHaveCount(1)
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
 expect(errors).toEqual([])
})

test('transaction rule can optionally assign its merchant without backfilling on save',async({page})=>{
 await page.goto('/');await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password');await page.getByRole('button',{name:'Sign in',exact:true}).click();await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible();
 await page.getByRole('navigation',{name:'Main navigation',exact:true}).getByRole('button',{name:'Transactions',exact:true}).click();
 await page.getByRole('tab',{name:'All transactions',exact:true}).click();
 await page.locator('.transaction-detail').filter({hasText:'Synthetic long description'}).click();
 const editor=page.getByRole('dialog',{name:/^Transaction #[0-9]+$/});
 await editor.getByLabel('Category',{exact:true}).click();await page.getByRole('dialog',{name:'Select category',exact:true}).getByRole('button').filter({hasText:'Groceries'}).first().click();
 await editor.locator('summary').filter({hasText:'Notes, merchant and tags'}).click();
 await editor.getByRole('button',{name:'New merchant',exact:true}).click();
 const create=page.getByRole('dialog',{name:'New merchant',exact:true});await create.getByLabel('Name',{exact:true}).fill('Optional transaction merchant');await create.getByRole('button',{name:'Create and select',exact:true}).click();await expect(create).toBeHidden();
 await editor.locator('summary').filter({hasText:'Automatically categorize similar transactions'}).click();await editor.getByLabel('Use this category and spending group for similar transactions in this account',{exact:true}).check();
 await editor.getByLabel("Assign this transaction's merchant to future matches",{exact:true}).check();await editor.getByLabel('Description contains',{exact:true}).fill('Optional merchant rule');
 const before=await(await page.request.get('/api/transactions')).json();
 await editor.getByRole('button',{name:'Save changes',exact:true}).click();const offer=page.getByRole('dialog',{name:'Apply this rule to previous transactions?',exact:true});await expect(offer).toBeVisible();await offer.getByRole('button',{name:'Not now',exact:true}).click();
 const rules=(await(await page.request.get('/api/merchant-rules?q=Optional%20merchant%20rule')).json()).items;expect(rules).toHaveLength(1);expect(rules[0].category_id).toBe(1);
 const after=await(await page.request.get('/api/transactions')).json();
 expect(after.items.filter((t:any)=>t.version!==before.items.find((b:any)=>b.id===t.id)?.version)).toHaveLength(1);
});
