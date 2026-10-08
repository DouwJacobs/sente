import {test,expect} from '@playwright/test'
async function login(page:any){
 await page.goto('/')
 await page.getByLabel('Username',{exact:true}).fill('demo')
 await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password')
 await page.getByRole('button',{name:'Sign in',exact:true}).click()
 await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
}
async function navigate(page:any,name:string,mobile=false){
 if(mobile){await page.getByRole('navigation',{name:'Mobile navigation'}).getByRole('button',{name:'More',exact:true}).click();await page.getByRole('navigation',{name:'More pages'}).getByRole('button',{name,exact:true}).click()}
 else await page.getByRole('navigation',{name:'Main navigation',exact:true}).getByRole('button',{name,exact:true}).click()
}
test('branding validation, stale edits, persistence and long names',async({page,browser})=>{
 await login(page);await navigate(page,'Settings');await page.getByRole('tab',{name:'Branding',exact:true}).click()
 const field=page.getByLabel('Display name',{exact:true})
 await field.fill(' ');await expect(field).not.toHaveAttribute('aria-invalid','true')
 await page.getByRole('button',{name:'Save branding',exact:true}).focus()
 await expect(field).toHaveAttribute('aria-invalid','true')
 const longName='A long family workspace name for navigation layout checks'
 await field.fill(longName)
 await expect(field).not.toHaveAttribute('aria-invalid','true')
 const second=await browser.newPage();await login(second);await navigate(second,'Settings');await second.getByRole('tab',{name:'Branding',exact:true}).click()
 await page.getByRole('button',{name:'Save branding',exact:true}).click()
 await expect(page).toHaveTitle(longName+' · Sente')
 await second.getByLabel('Display name').fill('Stale name')
 await second.getByRole('button',{name:'Save branding',exact:true}).click()
 await expect(second.getByRole('alert')).toBeVisible()
 await second.getByRole('button',{name:'Reload saved branding',exact:true}).click()
 await expect(second.getByLabel('Display name')).toHaveValue(longName)
 await second.close()
 await page.reload();await expect(page).toHaveTitle(longName+' · Sente')
 await navigate(page,'Settings')
 for(const width of [1280,360]){
  await page.setViewportSize({width,height:800})
  for(const theme of ['light','dark']){
   await page.getByLabel('Appearance',{exact:true}).selectOption(theme)
   expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
  }
 }
 await page.getByRole('button',{name:'Sign out',exact:true}).click()
 await expect(page).toHaveTitle('Sente')
 await expect(page.getByText(longName,{exact:true})).toHaveCount(0)
})
test('budget category creation preserves unsaved limits and chosen period',async({page})=>{
 await login(page);await navigate(page,'Budgets')
 await page.getByRole('button',{name:'Edit budgets',exact:true}).first().click()
 const dialog=page.getByRole('dialog').first()
 await dialog.getByLabel('Groceries',{exact:true}).fill('1234.56')
 await dialog.getByRole('button',{name:'Add category to No spending group',exact:true}).click()
 const add=page.getByRole('dialog',{name:'Add category · No spending group',exact:true})
 await add.getByRole('button',{name:'Create expense category',exact:true}).click()
 await add.getByLabel('Category name').fill('   ')
 await add.getByRole('button',{name:'Create category',exact:true}).focus()
 await expect(add.getByLabel('Category name')).toHaveAttribute('aria-invalid','true')
 await add.getByLabel('Category name').fill('Groceries')
 await add.getByRole('button',{name:'Create category',exact:true}).click()
 await expect(add.getByText('Category already exists',{exact:true})).toBeVisible()
 const name='Budget inline synthetic category'
 await add.getByLabel('Category name').fill(name)
 await expect(add.getByLabel('Type')).toBeDisabled()
 await add.getByRole('button',{name:'Create category',exact:true}).click()
 await expect(add.getByLabel('Budget amount')).toHaveValue('0.00')
 await add.getByLabel('Budget amount').fill('75.29')
 await add.getByRole('button',{name:'Add category to budget',exact:true}).click()
 await expect(dialog.getByLabel('Groceries',{exact:true})).toHaveValue('1234.56')
 await expect(dialog.getByLabel(name,{exact:true})).toHaveValue('75.29')
 await dialog.getByRole('button',{name:'Save budget',exact:true}).click()
 await expect(dialog).not.toBeVisible()
 await page.reload();await navigate(page,'Budgets')
 await page.getByRole('button',{name:'Edit budgets',exact:true}).first().click()
 await expect(page.getByRole('dialog').getByLabel(name,{exact:true})).toHaveValue('75.29')
 await expect(page.getByRole('dialog').getByLabel('Groceries',{exact:true})).toHaveValue('1234.56')
})
