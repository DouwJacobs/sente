
import {test,expect} from '@playwright/test'
for(const width of [1440,360])test('network saves and restarts at '+width+'px',async({page})=>{
 await page.setViewportSize({width,height:900})
 await page.goto('/')
 await page.getByLabel('Username',{exact:true}).fill('demo')
 await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password')
 await page.getByRole('button',{name:'Sign in',exact:true}).click()
 await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
 if(width===360){
  await page.getByRole('navigation',{name:'Mobile navigation'}).getByRole('button',{name:'More',exact:true}).click()
  await page.getByRole('navigation',{name:'More pages'}).getByRole('button',{name:'Settings',exact:true}).click()
 }else await page.getByRole('navigation',{name:'Main navigation',exact:true}).getByRole('button',{name:'Settings',exact:true}).click()
 await page.getByRole('tab',{name:'Network',exact:true}).click()
 const url=page.getByLabel('Public URL',{exact:true})
 await expect(url).toBeVisible()
 const original=await url.inputValue()
 await url.fill('https://example.test/path')
 await url.blur()
 await expect(page.getByText('Enter a full http:// or https:// address', {exact:false})).toBeVisible()
 await expect(page.getByRole('button',{name:'Restart tracker',exact:true})).toBeDisabled()
 await url.fill(original)
 await page.getByLabel('Trusted proxy addresses',{exact:true}).fill(width===1440?'127.0.0.1':'127.0.0.2')
 await page.getByRole('button',{name:'Save network settings',exact:true}).click()
 await expect(page.getByText('Restart required',{exact:true})).toBeVisible()
 await page.getByRole('button',{name:'Restart tracker',exact:true}).click()
 await expect(page.getByRole('link',{name:'Open tracker at '+original})).toHaveAttribute('href',original)
 await page.waitForTimeout(1000)
 await expect.poll(async()=>{
  try {const r=await page.request.get('/api/network');const s=await r.json();return s.restart_required===false&&s.active.trusted_proxies===(width===1440?'127.0.0.1':'127.0.0.2')}catch{return false}
 },{timeout:15000}).toBeTruthy()
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy()
})
