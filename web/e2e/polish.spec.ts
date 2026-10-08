import {test,expect,type Page} from '@playwright/test'

async function navigate(page:Page,name:string,mobile=false){
 const tab=name==='Review'?'Needs review':name==='Imports'?'Import activity':''
 const target=tab?'Transactions':name
 if(mobile&&!['Dashboard','Transactions','Review'].includes(target)){await page.getByRole('navigation',{name:'Mobile navigation'}).getByRole('button',{name:'More',exact:true}).click();await page.getByRole('navigation',{name:'More pages'}).getByRole('button',{name:target,exact:true}).click()}
 else await page.getByRole('navigation',{name:mobile?'Mobile navigation':'Main navigation',exact:true}).getByRole('button',{name:target,exact:true}).click()
 await expect(page.getByRole('heading',{name:target,exact:true,level:1})).toBeVisible()
 if(tab)await page.getByRole('tab',{name:new RegExp(tab)}).click();else if(name==='Transactions')await page.getByRole('tab',{name:'All transactions',exact:true}).click()
 if(name==='Categories')await page.getByRole('tab',{name:'Automatic rules',exact:true}).click()
}
async function fit(page:Page){
 await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
 const bounds=await page.locator('.panel:visible').evaluateAll(panels=>panels.map(panel=>{
  const box=panel.getBoundingClientRect();return {left:box.left,right:box.right,width:box.width}
 }))
 for(const box of bounds){expect(box.width).toBeGreaterThan(0);expect(box.left).toBeGreaterThanOrEqual(0);expect(box.right).toBeLessThanOrEqual(page.viewportSize()!.width+1)}
}
for(const width of [1440,900,360])for(const theme of ['light','dark'])test(`whole-app layout and keyboard contracts at ${width}px in ${theme}`,async({page})=>{
 await page.setViewportSize({width,height:900})
 await page.addInitScript(theme=>localStorage.setItem('finance-theme',theme),theme)
 await page.route('**/api/branding',route=>route.fulfill({json:{display_name:'A deliberately long synthetic household workspace name',version:1}}))
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message))
 await page.goto('/')
 await page.getByLabel('Username',{exact:true}).fill('demo')
 await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password')
 await page.getByRole('button',{name:'Sign in',exact:true}).click()
 await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
 await expect(page.locator('html')).toHaveAttribute('data-theme',theme)
 await expect(page.locator('.stats')).toBeVisible()
 // The colored finance design remains shared and responsive.
 expect(await page.locator('html').evaluate(el=>getComputedStyle(el).backgroundColor)).toBe(theme==='light'?'rgb(245, 245, 246)':'rgb(21, 23, 28)')
 expect(await page.locator('#page-title').evaluate(el=>getComputedStyle(el).fontSize)).toBe(width===360?'30px':'36px')
 expect(await page.locator('.panel').first().evaluate(el=>getComputedStyle(el).borderRadius)).toBe('14px')
 const cards=await page.locator('.stats .stat').evaluateAll(elements=>elements.map(el=>{const box=el.getBoundingClientRect();return {width:box.width,top:box.top,bottom:box.bottom}}))
 expect(cards).toHaveLength(4)
 if(width===1440){expect(cards[0].width).toBeGreaterThan(cards[1].width);for(const card of cards.slice(1))expect(Math.abs(card.width-cards[1].width)).toBeLessThan(1)}
 else for(const card of cards)expect(Math.abs(card.width-cards[0].width)).toBeLessThan(1)
 for(let i=0;i<cards.length;i+=width===1440?4:2){const row=cards.slice(i,i+(width===1440?4:2));for(const card of row){expect(Math.abs(card.top-row[0].top)).toBeLessThan(1);expect(Math.abs(card.bottom-row[0].bottom)).toBeLessThan(1)}}
 const contrast=await page.locator('.page-head .muted,.stat-heading>span:first-child,.stat>strong,.footnote').evaluateAll(elements=>elements.map(element=>{
  const rgb=(value:string)=>value.match(/[\d.]+/g)!.slice(0,3).map(Number)
  const luminance=(color:number[])=>color.map(value=>{value/=255;return value<=.04045?value/12.92:((value+.055)/1.055)**2.4}).reduce((sum,value,index)=>sum+value*[.2126,.7152,.0722][index],0)
  let parent:Element|null=element,background='rgba(0, 0, 0, 0)'
  while(parent){background=getComputedStyle(parent).backgroundColor;if(background!=='rgba(0, 0, 0, 0)'&&background!=='transparent')break;parent=parent.parentElement}
  const foreground=luminance(rgb(getComputedStyle(element).color)),backdrop=luminance(rgb(background))
  return (Math.max(foreground,backdrop)+.05)/(Math.min(foreground,backdrop)+.05)
 }))
 for(const ratio of contrast)expect(ratio).toBeGreaterThanOrEqual(4.5)
 await page.emulateMedia({reducedMotion:'reduce'})
 await expect.poll(()=>page.locator('.button').first().evaluate(el=>getComputedStyle(el).transitionDuration)).toBe('0s')
 await page.emulateMedia({reducedMotion:'no-preference'})
 await fit(page)
 // Heading precedes content without a permanent blank loading row.
 await expect(page.locator('main>.workspace-status')).toHaveCount(0)
 const textSize=await page.locator('.footnote').first().evaluate(el=>parseFloat(getComputedStyle(el).fontSize))
 expect(textSize).toBeGreaterThanOrEqual(12)
 const mobile=width===360
 for(const name of ['Transactions','Accounts','Budgets','Categories','Settings']){
  await navigate(page,name,mobile);await fit(page)
  await expect(page.locator('#page-title')).toHaveText(name)
 }
 // All settings sections share responsive tabs and panels, with roving focus.
 for(const name of ['General','Banking','Accounts','Users & access','Backups','Network','Security']){
  await page.getByRole('tab',{name,exact:true}).click()
  await expect(page.getByRole('tabpanel',{name,exact:true})).toBeVisible()
  await fit(page)
 }
 await page.getByRole('tab',{name:'General',exact:true}).focus()
 await page.keyboard.press('End')
 await expect(page.getByRole('tab',{name:'About',exact:true})).toBeFocused()
 await page.keyboard.press('Home')
 await expect(page.getByRole('tab',{name:'General',exact:true})).toBeFocused()
 // Skip navigation is available to keyboard users and moves to real content.
 await page.locator('.skip-link').focus();await expect(page.locator('.skip-link')).toBeInViewport()
 await page.keyboard.press('Enter');await expect(page.locator('#main-content')).toBeFocused()
 await navigate(page,'Budgets',mobile)
 await page.getByLabel(/Actions for/).first().click()
 await page.getByRole('button',{name:'Edit dates',exact:true}).first().click()
 const dialog=page.getByRole('dialog')
 await expect(dialog).toBeVisible();await fit(page)
 const box=(await dialog.boundingBox())!;expect(box.x).toBeGreaterThanOrEqual(0);expect(box.x+box.width).toBeLessThanOrEqual(width+1)
 await dialog.getByLabel('Period name').fill('')
 await dialog.getByRole('button',{name:'Preview affected transactions',exact:true}).click()
 await expect(dialog.getByLabel('Period name')).toHaveAttribute('aria-invalid','true')
 await expect(dialog.getByLabel('Period name')).toBeFocused()
 await fit(page)
 await dialog.getByRole('button',{name:'Cancel',exact:true}).click();await expect(dialog).not.toBeVisible()
 // Empty content keeps the same panel tracks and usable page navigation.
 await page.route('**/api/dashboard?**',async route=>{
  const response=await route.fetch(),body=await response.json()
  await route.fulfill({json:{...body,categories:[],category_total:0,spending_groups:[],group_total:0,income_categories:[],income_category_total:0,balances:[],balance_total:0,uncategorized_count:0}})
 })
 await navigate(page,'Dashboard',mobile)
 await expect(page.getByRole('heading',{name:'No spending yet',exact:true})).toBeVisible()
 await expect(page.getByRole('heading',{name:'No balances for these accounts',exact:true})).toBeVisible()
 await fit(page)
 await page.route('**/api/transactions?**',route=>route.fulfill({json:{items:[],total:0,more:false,list_version:'synthetic-empty'}}))
 await navigate(page,'Review',mobile)
 await expect(page.getByRole('heading',{name:'No transactions found',exact:true})).toBeVisible()
 await fit(page)
 await navigate(page,'Transactions',mobile)
 await expect(page.getByRole('heading',{name:'No transactions found',exact:true})).toBeVisible()
 await fit(page)
 expect(errors).toEqual([])
})
