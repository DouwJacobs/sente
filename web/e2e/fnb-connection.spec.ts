import {test,expect} from '@playwright/test'
for(const width of [1440,360])test(`FNB connection controls preserve secrecy and explicit refresh choices at ${width}px`,async({page})=>{
 await page.setViewportSize({width,height:900})
 let connection:any=null
 const discovered:any[]=[{bank_id:'00123456',name:'Synthetic discovered account',balance_cents:123456,balance_date:'2026-10-02',hidden:0,account_id:99}]
 let connects=0,refreshes=0,schedules=0
 await page.route('**/api/fnb**',async route=>{
  const request=route.request(),path=new URL(request.url()).pathname
  if(path==='/api/fnb'&&request.method()==='GET'){await route.fulfill({json:{connection,accounts:connection?discovered:[]}});return}
  if(path==='/api/fnb'&&request.method()==='PUT'){
   const body=request.postDataJSON();expect(body.username).toBe('synthetic-bank-user');expect(body.password).toBe('synthetic-bank-secret');connects++
   connection={interval_hours:0,state:'ready',last_error:'',debug_browser:0,version:1};await route.fulfill({json:{ok:true}});return
  }
  if(path==='/api/fnb/debug'){
   const body=request.postDataJSON();expect(body.version).toBe(connection.version)
   connection={...connection,debug_browser:body.debug_browser?1:0,version:connection.version+1};await route.fulfill({json:{ok:true}});return
  }
  if(path==='/api/fnb/refresh'){
   refreshes++;connection={...connection,state:'action_required',last_error:'BALANCE_LAYOUT_CHANGED',last_diagnostics:{ledger_nodes:12,missing_rows:1},version:connection.version+1}
   await route.fulfill({status:502,json:{error:'FNB refresh needs attention: BALANCE_LAYOUT_CHANGED'}});return
  }
  if(path==='/api/fnb/schedule'){
   const body=request.postDataJSON();expect(body.interval_hours).toBe(24);expect(body.version).toBe(connection.version);schedules++;connection={...connection,interval_hours:24,version:connection.version+1};await route.fulfill({json:{ok:true}});return
  }
  if(path==='/api/fnb/accounts'){
   const body=request.postDataJSON();expect(body.bank_id).toBe('00123456');discovered[0].hidden=body.hidden?1:0;await route.fulfill({json:{ok:true}});return
  }
  await route.fulfill({json:{ok:true}})
 })
 await page.goto('/')
 await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password')
 await page.getByRole('button',{name:'Sign in',exact:true}).click()
 await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
 if(width===360){await page.getByRole('navigation',{name:'Mobile navigation'}).getByRole('button',{name:'More',exact:true}).click();await page.getByRole('navigation',{name:'More pages'}).getByRole('button',{name:'Settings',exact:true}).click()}
 else await page.getByRole('navigation',{name:'Main navigation',exact:true}).getByRole('button',{name:'Settings',exact:true}).click()
 await page.getByRole('tab',{name:'Banking',exact:true}).click()
 const panel=page.locator('section.panel').filter({has:page.getByRole('heading',{name:'FNB connection',exact:true})})
 const password=panel.getByLabel('FNB password',{exact:true})
 await expect(password).not.toHaveAttribute('aria-invalid','true')
 await panel.getByLabel('FNB username',{exact:true}).fill('synthetic-bank-user')
 await panel.getByRole('button',{name:'Connect FNB',exact:true}).click()
 await expect(password).toBeFocused();expect(connects).toBe(0)
 await password.fill('synthetic-bank-secret');await panel.getByRole('button',{name:'Connect FNB',exact:true}).click()
 await expect(panel.getByRole('button',{name:'Refresh now',exact:true})).toBeVisible()
 await expect(password).toHaveCount(0);expect(connects).toBe(1)
 await expect(panel.getByLabel('Refresh schedule',{exact:true})).not.toBeVisible()
 await expect(panel.getByRole('button',{name:'Disconnect',exact:true})).not.toBeVisible()
 await panel.locator('summary').filter({hasText:'Automatic refresh'}).click()
 await expect(panel.getByLabel('Refresh schedule',{exact:true})).toHaveValue('0')
 await expect(panel.getByText('Layout diagnostics (counts only)',{exact:true})).not.toBeVisible()
 await panel.locator('summary').filter({hasText:'Troubleshooting'}).focus()
 await page.keyboard.press('Enter')
 await expect(panel.getByText('Layout diagnostics (counts only)',{exact:true})).toBeVisible()
 const debug=panel.getByRole('checkbox',{name:'Debug mode — show Chrome during refresh'})
 await expect(debug).not.toBeChecked()
 await debug.check();await panel.getByRole('button',{name:'Save browser mode',exact:true}).click()
 await expect(debug).toBeChecked()
 await page.reload()
 if(width===360){await page.getByRole('navigation',{name:'Mobile navigation'}).getByRole('button',{name:'More',exact:true}).click();await page.getByRole('navigation',{name:'More pages'}).getByRole('button',{name:'Settings',exact:true}).click()}
 else await page.getByRole('navigation',{name:'Main navigation',exact:true}).getByRole('button',{name:'Settings',exact:true}).click()
 await page.getByRole('tab',{name:'Banking',exact:true}).click()
 await panel.locator('summary').filter({hasText:'Troubleshooting'}).click()
 await expect(debug).toBeChecked()
 await panel.getByRole('button',{name:'Refresh now',exact:true}).click()
 await expect(page.locator('.toast:popover-open')).toContainText('BALANCE_LAYOUT_CHANGED');expect(refreshes).toBe(1)
  await expect(panel.locator('code')).toContainText('BALANCE_COUNTS: {"ledger_nodes":12,"missing_rows":1}')
 await debug.uncheck();await panel.getByRole('button',{name:'Save browser mode',exact:true}).click();await expect(debug).not.toBeChecked()
 await panel.locator('summary').filter({hasText:'Automatic refresh'}).click()
 await panel.getByLabel('Refresh schedule',{exact:true}).selectOption('24')
 await panel.getByRole('button',{name:'Save refresh schedule',exact:true}).click()
 await expect(panel.getByLabel('Refresh schedule',{exact:true})).toHaveValue('24');expect(schedules).toBe(1)
 await expect(panel.locator('summary').filter({hasText:'Account visibility'})).toHaveCount(0)
 await panel.locator('summary').filter({hasText:'Connection settings'}).click()
 await panel.getByRole('button',{name:'Replace credentials',exact:true}).click()
 await expect(panel.getByLabel('FNB password',{exact:true})).toHaveValue('')
 await panel.getByLabel('FNB password',{exact:true}).fill('discarded-synthetic-secret')
 await panel.getByRole('button',{name:'Cancel',exact:true}).click()
 await expect(panel.getByLabel('FNB password',{exact:true})).toHaveCount(0)
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy()
 expect(await page.evaluate(()=>Object.values(localStorage).some(value=>value.includes('synthetic-bank-secret')))).toBeFalsy()
})
