import {test,expect} from '@playwright/test'
for(const width of [1440,360])test(`account-to-banking navigation and adjacent keyboard tabs at ${width}px`,async({page})=>{
 await page.setViewportSize({width,height:900})
 await page.goto('/')
 await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password')
 await page.getByRole('button',{name:'Sign in',exact:true}).click()
 await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
 async function navigate(name:string){if(width===360){await page.getByRole('navigation',{name:'Mobile navigation'}).getByRole('button',{name:'More',exact:true}).click();await page.getByRole('navigation',{name:'More pages'}).getByRole('button',{name,exact:true}).click()}else await page.getByRole('navigation',{name:width===360?'Mobile navigation':'Main navigation',exact:true}).getByRole('button',{name,exact:true}).click()}
 await navigate('Accounts')
 await expect(page.getByRole('heading',{name:'FNB connection',exact:true})).toHaveCount(0)
 await page.getByLabel('Account management',{exact:true}).click()
 await expect(page.getByRole('button',{name:'Add account',exact:true})).toBeVisible()
 await page.getByRole('button',{name:'Bank connection',exact:true}).click()
 await expect(page.getByRole('heading',{name:'Settings',exact:true})).toBeVisible()
 await expect(page.getByRole('tab',{name:'Banking',exact:true})).toHaveAttribute('aria-selected','true')
 await expect(page.getByRole('heading',{name:'FNB connection',exact:true})).toBeVisible()
 await page.getByRole('tab',{name:'Branding',exact:true}).click()
 // Draft retention and Home/End focus live in the responsive workflow owner.
 const branding=page.getByRole('tab',{name:'Branding',exact:true})
 await branding.focus();await page.keyboard.press('ArrowRight')
 await expect(page.getByRole('tab',{name:'Configuration',exact:true})).toBeFocused()
 await page.getByRole('tab',{name:'Security',exact:true}).click()
 await expect(page.getByRole('heading',{name:'Change your password',exact:true})).toBeVisible()
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy()
 await expect(page.getByRole('button',{name:'Change appearance',exact:true})).toHaveCount(0)
})

for(const width of [360,1440])test('ordinary users see General, PWA, MCP, Notifications, Security and About without management requests at '+width+'px',{tag:width===360?'@mobile-smoke':[]},async({page})=>{
 await page.setViewportSize({width,height:800});
 const management:string[]=[]
 page.on('request',request=>{if(/\/api\/(users|grants|backups|accounts\/manage|fnb)(?:$|\?)/.test(new URL(request.url()).pathname))management.push(request.url())})
 await page.route('**/api/me',async route=>{const response=await route.fetch();if(response.status()===200){const body=await response.json();await route.fulfill({response,json:{...body,user:{...body.user,admin:false}}})}else await route.fulfill({response})})
 await page.goto('/')
 await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password')
 await page.getByRole('button',{name:'Sign in',exact:true}).click()
 await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
 if(width===360)await page.getByRole('navigation',{name:'Mobile navigation',exact:true}).getByRole('button',{name:'More',exact:true}).click()
 await page.getByRole('navigation',{name:width===360?'More pages':'Main navigation',exact:true}).getByRole('button',{name:'Settings',exact:true}).click()
 for(const name of ['General','PWA','MCP','Notifications','Security','About']){
  const tab=page.getByRole('tab',{name,exact:true});await expect(tab).toBeVisible();await tab.click()
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
 }
 for(const name of ['Branding','Configuration','Accounts','Users & access','Backups','Network','Banking']){
  await expect(page.getByRole('tab',{name,exact:true})).toHaveCount(0)
 }
 await expect(page.getByRole('tab',{name:'Diagnostics',exact:true})).toHaveCount(0)
 await expect(page.getByRole('tab',{name:'MCP',exact:true})).toBeVisible()
 await page.getByRole('tab',{name:'Security',exact:true}).click()
 await expect(page.getByLabel('Current password',{exact:true})).toBeVisible()
 expect(management).toEqual([])
})
