import { colorToken } from './responsive-helpers';
import {test,expect} from '@playwright/test'
for(const theme of ['light','dark'])test(`over-limit category presentation retains account scope in ${theme}`,async({page})=>{
 await page.addInitScript(theme=>localStorage.setItem('finance-theme',theme),theme)
 await page.route('**/api/dashboard?**',async route=>{
  const response=await route.fetch(),body=await response.json(),account=new URL(route.request().url()).searchParams.get('account')
  await route.fulfill({json:{...body,has_targets:!account,categories:[{...body.categories[0],id:1,name:'Synthetic budget category',kind:'expense',target_cents:10000,spent_cents:12500,pending_cents:0}],category_total:1}})
 })
 await page.goto('/');await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password');await page.getByRole('button',{name:'Sign in',exact:true}).click()
 await page.locator('.category-limits-summary > summary').click()
 const row=page.locator('.category-row').filter({hasText:'Synthetic budget category'})
 await expect(row).toBeVisible();await expect(row.getByText(/25[.,]00 over limit/)).toBeVisible()
 const progress=row.getByRole('progressbar');await expect(progress).toHaveClass('over-budget');await expect(progress).toHaveAttribute('max','10000');await expect(progress).toHaveAttribute('value','12500')
 await expect(row.locator('.negative')).toHaveCSS('color',await colorToken(page,'--negative'))
 await page.getByLabel('Accounts',{exact:true}).selectOption('1')
 await expect(row.getByText(/over limit/)).toHaveCount(0);await expect(row.getByRole('progressbar')).toHaveCount(0)
 await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
})
