import {test,expect} from '@playwright/test'
for(const width of [360,390,780,1440])test(`calm banking overview explains and recovers a paused 05:00 schedule at ${width}px`,async({page})=>{
 await page.setViewportSize({width,height:900})
 await page.addInitScript(theme=>localStorage.setItem('finance-theme',theme),width===390||width===1440?'dark':'light')
 await page.clock.install()
 let connection:any={state:'action_required',last_error:'INTERRUPTED',schedule:{frequency:'daily',times:['05:00']},interval_hours:0,daily_time:'',next_due:null,version:1}
 let refreshes=0
 await page.route('**/api/fnb**',async route=>{
  const request=route.request()
  if(request.method()==='GET'){await route.fulfill({json:{connection,accounts:[]}});return}
  expect(new URL(request.url()).pathname).toBe('/api/fnb/refresh')
  refreshes++
  if(refreshes===1){connection={...connection,last_error:'APPROVAL_REQUIRED',version:connection.version+1};await route.fulfill({status:502,json:{error:'FNB approval required'}});return}
  connection={...connection,state:'ready',last_error:'',next_due:Date.UTC(2026,9,16,3)/1000,version:connection.version+1}
  await route.fulfill({json:{ok:true}})
 })
 await page.goto('/')
 await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password')
 await page.getByRole('button',{name:'Sign in',exact:true}).click()
 await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
 if(width<=390){await page.getByRole('navigation',{name:'Mobile navigation',exact:true}).getByRole('button',{name:'More',exact:true}).click();await page.getByRole('navigation',{name:'More pages',exact:true}).getByRole('button',{name:'Settings',exact:true}).click()}
 else await page.getByRole('navigation',{name:'Main navigation',exact:true}).getByRole('button',{name:'Settings',exact:true}).click()
 await page.getByRole('tab',{name:'Accounts & banking',exact:true}).click()
 const tab=page.getByRole('tabpanel',{name:'Accounts & banking',exact:true}),panel=tab.locator('.banking-settings')
 const pause=panel.getByRole('region',{name:'Connection needs attention'})
 await expect(tab.locator('section.panel')).toHaveCount(1)
 await expect(pause).toContainText('The previous bank update was interrupted when Sente stopped or restarted.')
 await expect(pause).toContainText('Your schedule is saved and resumes after a successful refresh.')
 await expect(panel.getByLabel('Refresh schedule',{exact:true})).not.toBeVisible()
 await expect(panel.getByRole('checkbox',{name:'Show Chrome during bank updates'})).not.toBeVisible()
 await expect(panel.getByRole('button',{name:'Disconnect',exact:true})).not.toBeVisible()
 await expect(panel.locator('.banking-group:visible')).toHaveCount(2)
 await expect(panel.locator('summary').filter({hasText:'Automatic refresh'})).toContainText('Every day at 05:00 SAST')
 const refresh=panel.getByRole('button',{name:'Refresh now',exact:true})
 await refresh.click();await expect(pause).toContainText('The last sign-in needed approval in your FNB app.')
 expect(refreshes).toBe(1)
 await refresh.click();await expect(pause).toHaveCount(0)
 await expect(panel.locator('.banking-overview')).toContainText('05:00')
 await expect(panel.locator('.banking-overview')).not.toContainText('Paused')
 expect(refreshes).toBe(2)
 connection={...connection,state:'refreshing'}
 await page.clock.fastForward(16000)
 await expect(panel.locator('.banking-overview')).toContainText('Update in progress')
 await expect(panel.locator('.banking-overview')).not.toContainText('Paused')
 await expect(refresh).toBeDisabled()
 connection={...connection,state:'action_required',last_error:'KEY_UNAVAILABLE'}
 await page.clock.fastForward(16000)
 await expect(pause).toContainText('restore the FNB encryption key')
 connection={...connection,last_error:'unrecognized provider code'}
 await page.clock.fastForward(16000)
 await expect(pause).toContainText('The bank connection needs attention')
 await expect(pause).not.toContainText('unrecognized provider code')
 for(const control of [refresh,panel.getByRole('button',{name:'Open Accounts',exact:true}),panel.locator('summary').filter({hasText:'Automatic refresh'})]){
  const bounds=await control.boundingBox();expect(bounds!.height).toBeGreaterThanOrEqual(44)
 }
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
 // Revealing optional settings is read-only and does not start another bank session.
 await panel.locator('summary').filter({hasText:'Connection options'}).click()
 await expect(panel.getByRole('checkbox',{name:'Show Chrome during bank updates'})).toBeVisible()
 await expect(panel.getByText('Layout diagnostics (counts only)',{exact:true})).not.toBeVisible()
 expect(refreshes).toBe(2)
})
