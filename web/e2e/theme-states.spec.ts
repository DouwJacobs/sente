import { colorToken } from './responsive-helpers';
import { desktopPhoneCases } from './coverage-cases';
import {test,expect} from '@playwright/test'
import {readFileSync} from 'node:fs'

const cssURL=readFileSync(new URL('../dist/index.html',import.meta.url),'utf8').match(/href="([^"]+\.css)"/)![1]
for(const {width,theme} of desktopPhoneCases)test(`shared interaction states at ${width}px in ${theme}`,async({page})=>{
 await page.setViewportSize({width,height:900})
 await page.route('**/theme-fixture.css',route=>route.fulfill({contentType:'text/css',body:'main{margin:0;width:100%;max-width:500px;padding:24px}'}))
 await page.route('**/theme-fixture',route=>route.fulfill({contentType:'text/html',headers:{'Content-Security-Policy':"default-src 'self'; style-src 'self'"},body:`<!doctype html><html><head><link rel="stylesheet" href="${cssURL}"/><link rel="stylesheet" href="/theme-fixture.css"/></head><body><main>
 <button data-check="secondary" class="button secondary">Secondary action</button>
 <button data-check="quiet" class="button quiet">Quiet action</button>
 <button data-check="danger" class="button danger">Delete</button>
 <button data-check="primary" class="button primary">Save</button>
 <button data-check="disabled" class="button secondary" disabled>Unavailable</button>
 <button data-check="nav" class="nav-item">Navigation</button>
 <button data-check="selected-nav" class="nav-item active" aria-current="page">Selected navigation</button>
 <div class="settings-tabs"><button data-check="tab" class="button quiet" role="tab" aria-selected="false">Inactive tab</button><button data-check="selected-tab" class="button primary" role="tab" aria-selected="true">Selected tab</button></div>
 <button data-check="action" class="action-row">Dashboard action</button>
 <div data-check="transaction" class="transaction-row"><button class="transaction-detail">Transaction</button></div>
 <details><summary data-check="disclosure">Disclosure</summary><p>Details</p></details>
 <button data-check="choice" class="choice-row">Category option</button>
 <button data-check="picker" class="choice-control">Category picker</button>
 <label class="field">Name<input data-check="input" value="Synthetic name"/></label>
 <button data-check="upload" class="file-picker">Choose files</button>
 </main></body></html>`}))
 await page.goto('/theme-fixture');await page.evaluate(theme=>document.documentElement.dataset.theme=theme,theme)
 const colorContrast=async(selector:string)=>page.locator(selector).evaluate(el=>{
  const numbers=(value:string)=>value.match(/[\d.]+/g)!.slice(0,3).map(Number)
  const light=(rgb:number[])=>rgb.map(n=>{n/=255;return n<=.04045?n/12.92:((n+.055)/1.055)**2.4}).reduce((sum,n,i)=>sum+n*[.2126,.7152,.0722][i],0)
  const css=getComputedStyle(el),a=light(numbers(css.color)),b=light(numbers(css.backgroundColor))
  return (Math.max(a,b)+.05)/(Math.min(a,b)+.05)
 })
 for(const id of ['secondary','quiet','danger','nav','tab','action','transaction','disclosure','choice','picker','input','upload']){
  const selector=`[data-check="${id}"]`,target=page.locator(selector)
  await page.mouse.move(0,0);const resting=await target.evaluate(el=>getComputedStyle(el).backgroundColor)
  await target.hover()
  const token=['input','picker'].includes(id)?'--surface-soft':id==='danger'?'--danger-bg':'--interaction-hover'
  await expect(target).toHaveCSS('background-color',await colorToken(page,token))
  expect(await target.evaluate(el=>getComputedStyle(el).backgroundColor)).not.toBe(resting)
  expect(await colorContrast(selector)).toBeGreaterThanOrEqual(4.5)
 }
 for(const id of ['selected-nav','selected-tab']){
  const selected=page.locator(`[data-check="${id}"]`);await page.mouse.move(0,0)
  const before=await selected.evaluate(el=>getComputedStyle(el).backgroundColor)
  await selected.hover();await expect(selected).toHaveCSS('background-color',before)
 }
 const input=page.locator('[data-check="input"]')
 await page.mouse.move(0,0)
 const restingInputBorder=await input.evaluate(el=>getComputedStyle(el).borderColor)
 await input.evaluate(el=>el.setAttribute('aria-invalid','true'));await input.hover()
 await expect(input).toHaveCSS('border-color',await colorToken(page,'--negative'))
 expect(await input.evaluate(el=>getComputedStyle(el).borderColor)).not.toBe(restingInputBorder)
 await input.evaluate(el=>el.removeAttribute('aria-invalid'))
 const primary=page.locator('[data-check="primary"]');await primary.hover()
 await expect(primary).toHaveCSS('background-color',await colorToken(page,'--primary-hover'))
 expect(await colorContrast('[data-check="primary"]')).toBeGreaterThanOrEqual(4.5)
 const disabled=page.locator('[data-check="disabled"]');await page.mouse.move(0,0)
 const resting=await disabled.evaluate(el=>getComputedStyle(el).backgroundColor)
 await disabled.hover();await expect(disabled).toHaveCSS('background-color',resting)
 // Real keyboard input activates focus-visible independently of pointer hover.
 await page.mouse.move(0,0);await page.locator('[data-check="secondary"]').focus();await page.keyboard.press('Tab')
 const quiet=page.locator('[data-check="quiet"]');await expect(quiet).toBeFocused()
 expect(await quiet.evaluate(el=>el.matches(':focus-visible'))).toBe(true)
 expect(await quiet.evaluate(el=>getComputedStyle(el).outlineStyle)).not.toBe('none')
 expect(await quiet.evaluate(el=>parseFloat(getComputedStyle(el).outlineWidth))).toBeGreaterThan(0)
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
})
