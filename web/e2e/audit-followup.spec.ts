import {test,expect,type Page} from '@playwright/test'
async function go(page:Page,name:string,width:number){
 if(width===360&&!['Dashboard','Transactions','Review'].includes(name)){await page.getByRole('navigation',{name:'Mobile navigation'}).getByRole('button',{name:'More',exact:true}).click();await page.getByRole('navigation',{name:'More pages'}).getByRole('button',{name,exact:true}).click()}
 else await page.getByRole('navigation',{name:width===360?'Mobile navigation':'Main navigation',exact:true}).getByRole('button',{name,exact:true}).click()
}
async function login(page:Page){await page.goto('/');await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password');await page.getByRole('button',{name:'Sign in',exact:true}).click();await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()}
for(const width of [1440,360])for(const theme of ['light','dark'])test(`audit prerequisites and layout ${width} ${theme}`,async({page})=>{
 await page.setViewportSize({width,height:900});await page.addInitScript(t=>localStorage.setItem('finance-theme',t),theme);await login(page)
 const period=page.getByLabel('Budget period',{exact:true})
 await expect(period).toHaveValue('1');await expect(page.locator('.context-filter').first()).toContainText('2026-10-20 – 2026-11-19')
 const field=page.locator('.context-filter .field').first();await expect(field.locator('.field-error-space')).toHaveCount(1)
 const spacing=await page.locator('.context-bar').evaluate(el=>({padding:parseFloat(getComputedStyle(el).paddingTop),gap:parseFloat(getComputedStyle(el).gap)}));expect(spacing.padding).toBeLessThanOrEqual(12);expect(spacing.gap).toBeLessThanOrEqual(12)
 if(width===1440){const footer=await page.locator('.spending-panel > .panel-footer').boundingBox(),panel=await page.locator('.spending-panel').boundingBox();expect(panel!.y+panel!.height-footer!.y-footer!.height).toBeLessThan(30)}
 await page.evaluate(()=>window.scrollTo(0,document.body.scrollHeight));await go(page,'Settings',width);await expect.poll(()=>page.evaluate(()=>scrollY)).toBe(0)
 if(width===360){
  await page.getByRole('tab',{name:'Branding',exact:true}).click()
  const panel=page.getByRole('tabpanel',{name:'Branding',exact:true})
  const save=panel.getByRole('button',{name:'Save branding',exact:true})
  const reload=panel.getByRole('button',{name:'Reload saved branding',exact:true})
  await expect(save).toBeVisible();await expect(reload).toBeVisible()
  const first=await save.boundingBox(),second=await reload.boundingBox()
  expect(first).not.toBeNull();expect(second).not.toBeNull()
  expect(second!.y).toBeGreaterThanOrEqual(first!.y+first!.height)
  expect(Math.abs(first!.width-second!.width)).toBeLessThan(1)
 }
 await page.route('**/api/accounts**',route=>route.fulfill({json:{items:[],total:0,list_version:'empty-accounts'}}));await page.route('**/api/categories**',route=>route.fulfill({json:{items:[],total:0,list_version:'empty-categories'}}));await page.route('**/api/rules**',route=>route.fulfill({json:{items:[],total:0,list_version:'empty-rules'}}))
 await page.route('**/api/dashboard?**',async route=>{const response=await route.fetch(),body=await response.json();await route.fulfill({json:{...body,pending_count:0,unassigned_count:0,uncategorized_count:0,categories:[],category_total:0,balances:[],balance_total:0}})})
 await page.reload();await expect(page.getByRole('button',{name:/Set up accounts/})).toBeVisible();await expect(page.getByRole('button',{name:/Review transactions/})).toHaveCount(0)
 await page.getByRole('button',{name:/Set up accounts/}).click();await expect(page.getByRole('heading',{name:'Accounts',exact:true})).toBeVisible();await expect(page.getByRole('navigation',{name:'List pages'})).toHaveCount(0)
 await go(page,'Categories',width)
 await page.getByRole('tab',{name:'Automatic rules',exact:true}).click()
 await expect(page.getByRole('heading',{name:'No rules yet',exact:true})).toBeVisible()
 await expect(page.getByRole('button',{name:'Add rule',exact:true})).toHaveCount(0)
 await expect(page.getByText('Rules need an account you can edit and a category to match.',{exact:true})).toBeVisible()
 await page.getByRole('button',{name:'Open Accounts',exact:true}).click()
 await expect(page.getByRole('heading',{name:'Accounts',exact:true})).toBeVisible()

 await page.unroute('**/api/accounts**')
 await page.reload()
 await go(page,'Categories',width)
 await page.getByRole('tab',{name:'Automatic rules',exact:true}).click()
 await expect(page.getByRole('heading',{name:'No rules yet',exact:true})).toBeVisible()
 await expect(page.getByRole('button',{name:'Add rule',exact:true})).toHaveCount(0)
 await page.getByRole('button',{name:'Create category',exact:true}).click()
 await expect(page.getByRole('dialog',{name:'Add category',exact:true})).toBeVisible()
 await page.getByRole('dialog').getByRole('button',{name:'Close',exact:true}).click()
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
})
