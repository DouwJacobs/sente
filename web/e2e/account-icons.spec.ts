import { desktopPhoneCases } from './coverage-cases';
import {test,expect} from '@playwright/test'
for(const {width,theme} of desktopPhoneCases)test(`account type icons ${width} ${theme}`,async({page})=>{
 await page.setViewportSize({width,height:900});await page.addInitScript(t=>localStorage.setItem('finance-theme',t),theme)
 const types=['Credit','Savings','Home Loan','Cheque','']
 const items=types.map((account_type,i)=>({id:i+1,name:'Synthetic account '+(i+1)+(i===1?' with a long custom nickname':''),bank_id:String(12345678000+i),account_type,role:'editor',household:1,balance_cents:-12345,balance_date:'2026-10-04',version:1}))
 await page.route('**/api/accounts?**',route=>route.fulfill({json:{items,total:items.length,page:0,page_size:20,list_version:'synthetic'}}))
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message))
 await page.goto('/');await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password');await page.getByRole('button',{name:'Sign in',exact:true}).click()
 const nav=page.getByRole('navigation',{name:width===360?'Mobile navigation':'Main navigation',exact:true})
 if(width===360)await nav.getByRole('button',{name:'More',exact:true}).click()
 await page.getByRole('navigation',{name:width===360?'More pages':'Main navigation',exact:true}).getByRole('button',{name:'Accounts',exact:true}).click()
 const rows=page.locator('.account-summary');await expect(rows).toHaveCount(5)
 const labels=['Credit card account','Savings account','Home loan account','Transactional account','Account type unavailable']
 for(let i=0;i<labels.length;i++)await expect(rows.nth(i).getByRole('img',{name:labels[i],exact:true})).toBeVisible()
 await expect(rows.nth(1)).toContainText('long custom nickname')
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);expect(errors).toEqual([])
})
