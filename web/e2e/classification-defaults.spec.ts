import {test,expect,type Page} from '@playwright/test'
async function login(page:Page){
 await page.goto('/');await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password');await page.getByRole('button',{name:'Sign in',exact:true}).click();await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
}
async function navigate(page:Page,name:string,mobile=false){
 const tab=name==='Review'?'Needs review':name==='Imports'?'Import activity':''
 const target=tab?'Transactions':name
 if(mobile&&!['Dashboard','Transactions','Accounts'].includes(target)){await page.getByRole('navigation',{name:'Mobile navigation'}).getByRole('button',{name:'More',exact:true}).click();await page.getByRole('navigation',{name:'More pages'}).getByRole('button',{name:target,exact:true}).click()}
 else await page.getByRole('navigation',{name:mobile?'Mobile navigation':'Main navigation',exact:true}).getByRole('button',{name:target,exact:true}).click()
 await expect(page.getByRole('heading',{name:target,exact:true,level:1})).toBeVisible()
 if(tab)await page.getByRole('tab',{name:new RegExp(tab)}).click()
 if(name==='Categories')await page.getByRole('tab',{name:'Automatic rules',exact:true}).click()
}
for(const mobile of [false,true]){
 test('typed category and atomic transaction rule '+(mobile?'360px':'desktop'),async({page})=>{
  if(mobile)await page.setViewportSize({width:360,height:800})
  const category=mobile?'Workshop supplies mobile':'Workshop supplies desktop'
  const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message))
  await login(page);await navigate(page,'Review',mobile)
  await page.getByRole('button').filter({hasText:'Synthetic long description'}).click()
  const transaction=page.getByRole('dialog').filter({hasText:'Category amounts'})
  await transaction.getByLabel('Category',{exact:true}).click()
  const picker=page.getByRole('dialog',{name:'Select category',exact:true})
  const before=await page.evaluate(async()=> (await fetch('/api/categories').then(r=>r.json())).length)
  await picker.getByLabel('Search category',{exact:true}).fill(category)
  await expect(picker.getByRole('button',{name:'Create “'+category+'”',exact:true})).toBeVisible()
  const typed=await page.evaluate(async()=> (await fetch('/api/categories').then(r=>r.json())).length)
  expect(typed).toBe(before)
  // Enter creates and selects the typed name directly; there is no second category form.
  await picker.getByLabel('Search category',{exact:true}).press('Enter')
  await expect(picker).not.toBeVisible()
  await expect(transaction.getByLabel('Category',{exact:true})).toContainText(category)
  const after=await page.evaluate(async()=> (await fetch('/api/categories').then(r=>r.json())).length)
  expect(after).toBe(before+1)
  await transaction.getByLabel('Spending group',{exact:true}).click()
  await page.getByRole('dialog',{name:'Select spending group',exact:true}).getByRole('button',{name:/^Recurring(?: Selected)?$/}).click()
  await transaction.getByLabel('Use this category and spending group for similar transactions in this account',{exact:true}).check()
  await transaction.getByLabel('Description contains',{exact:true}).fill('Synthetic long description')
  await transaction.getByRole('button',{name:'Save changes',exact:true}).click()
  await expect(transaction).not.toBeVisible()
  const rule=await page.evaluate(async()=>{
   const rows=await fetch('/api/rules').then(r=>r.json());return rows.filter((r:{pattern:string})=>r.pattern==='Synthetic long description')
  })
  expect(rule).toHaveLength(1);expect(rule[0].category_name).toBe(category);expect(rule[0].spending_group_name).toBe('Recurring');expect(rule[0].direction).toBe('debit');expect(rule[0].account_id).toBe(1)
  await page.getByRole('tab',{name:'All transactions',exact:true}).click()
  await page.getByRole('button').filter({hasText:'Synthetic long description'}).click()
  await expect(page.getByRole('dialog').getByText('Accepted',{exact:true})).toBeVisible()
  // Restore the shared source fixture for the next independent category workflow.
  await transaction.getByLabel('Category',{exact:true}).click()
  await picker.getByRole('button',{name:/^Uncategorized/}).click()
  await transaction.getByRole('button',{name:'Save changes',exact:true}).click()
  await expect(transaction).not.toBeVisible()
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
  expect(errors).toEqual([])
 })
}
test('fresh installation ships editable defaults',async({page})=>{
 await page.goto('http://127.0.0.1:18082/')
 await page.getByLabel('Username',{exact:true}).fill('starter-owner')
 await page.getByLabel('Password',{exact:true}).fill('synthetic-starter-password')
 await page.getByLabel('Confirm password',{exact:true}).fill('synthetic-starter-password')
 await page.getByRole('button',{name:'Create account',exact:true}).click()
 await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
 await navigate(page,'Categories')
 const row=page.locator('.rule-row').filter({hasText:'Description contains “Monthly Account Fee”'})
 await expect(row.getByText('Built-in',{exact:true})).toBeVisible()
 await expect(row.getByText('Bank charges · Bank Fees',{exact:true})).toBeVisible()
 await row.getByRole('button',{name:'Pause',exact:true}).click()
 await expect(row.getByText('Paused',{exact:true})).toBeVisible()
 await row.getByRole('button',{name:'Resume',exact:true}).click()
 await expect(row.getByText('Paused',{exact:true})).toHaveCount(0)
 await row.getByRole('button',{name:'Edit rule Monthly Account Fee',exact:true}).click()
 const edit=page.getByRole('dialog',{name:'Edit rule',exact:true})
 await edit.getByLabel('Description contains',{exact:true}).fill('Monthly Account Fee updated')
 await edit.getByRole('button',{name:'Save rule',exact:true}).click()
 await expect(edit).not.toBeVisible()
 await expect(page.getByText('Description contains “Monthly Account Fee updated”',{exact:true})).toBeVisible()
 await page.getByRole('tab',{name:'Categories',exact:true}).click()
 await expect(page.getByText('Groceries',{exact:true})).toBeVisible()
 await page.getByRole('navigation',{name:'List pages'}).getByRole('button',{name:'Next',exact:true}).click()
 await expect(page.getByText('Salary',{exact:true})).toBeVisible()
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
})
