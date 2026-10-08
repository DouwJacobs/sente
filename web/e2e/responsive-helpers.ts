import { expect, test, type Page, type Locator } from '@playwright/test';

export const viewports = [
  { width:360, height:800 }, { width:390, height:844 }, { width:430, height:932 },
  { width:640, height:360 }, { width:900, height:720 }, { width:1440, height:900 },
];
export async function start(page:Page, viewport:{width:number;height:number}, theme:string, fixture:string) {
  await page.setViewportSize(viewport);
  await page.addInitScript(t=>localStorage.setItem('finance-theme',t),theme);
  await test.info().attach('responsive-fixture', {body:JSON.stringify({viewport,theme,fixture}),contentType:'application/json'});
  await page.goto('/');
  await page.getByLabel('Username',{exact:true}).fill('demo');
  await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password');
  await page.getByRole('button',{name:'Sign in',exact:true}).click();
  await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible();
}
export async function navigate(page:Page, name:string) {
  if(page.viewportSize()!.width<=760) {
    await page.getByRole('navigation',{name:'Mobile navigation',exact:true}).getByRole('button',{name:'More',exact:true}).click();
    await page.getByRole('navigation',{name:'More pages',exact:true}).getByRole('button',{name,exact:true}).click();
  } else await page.getByRole('navigation',{name:'Main navigation',exact:true}).getByRole('button',{name,exact:true}).click();
}
export async function contained(page:Page, target:Locator, name:string, touch=false) {
  const identity=await target.evaluate(e=>e.getAttribute('aria-label')||e.textContent?.trim().slice(0,80)||e.tagName);
  name+=` (${identity})`;
  const box=await target.boundingBox();
  expect(box,`${name}: has a box`).not.toBeNull();
  expect(box!.x,`${name}: left edge`).toBeGreaterThanOrEqual(-1);
  expect(box!.x+box!.width,`${name}: right edge`).toBeLessThanOrEqual(page.viewportSize()!.width+1);
  expect(await target.evaluate(e=>e.scrollWidth<=e.clientWidth+1),`${name}: content is not clipped`).toBe(true);
  if(touch) {
    expect(box!.width,`${name}: touch width`).toBeGreaterThanOrEqual(44);
    expect(box!.height,`${name}: touch height`).toBeGreaterThanOrEqual(44);
  }
}
export async function moneyFits(page:Page, target:Locator, name:string) {
  await contained(page,target,name);
  const geometry=await target.evaluate(e=>{
    const range=document.createRange();range.selectNodeContents(e);
    const text=range.getBoundingClientRect(),bounds=e.getBoundingClientRect();
    return {left:text.left-bounds.left,right:text.right-bounds.right,lines:range.getClientRects().length};
  });
  expect(geometry.lines,`${name}: complete amount on one line`).toBe(1);
  expect(geometry.left,`${name}: text starts inside`).toBeGreaterThanOrEqual(-1);
  expect(geometry.right,`${name}: text ends inside`).toBeLessThanOrEqual(1);
}
export async function noOverflow(page:Page) {
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth), 'page has no horizontal overflow').toBe(true);
}
export async function reachable(page:Page, target:Locator, name:string) {
  await target.scrollIntoViewIfNeeded();
  const box=(await target.boundingBox())!;
  expect(box.y,`${name}: above viewport top`).toBeGreaterThanOrEqual(0);
  expect(box.y+box.height,`${name}: below viewport bottom`).toBeLessThanOrEqual(page.viewportSize()!.height+1);
  const hit=await target.evaluate(e=>{
    const box=e.getBoundingClientRect(),hit=document.elementFromPoint(box.x+box.width/2,box.y+box.height/2);
    return hit===e||!!hit&&e.contains(hit);
  });
  expect(hit,`${name}: unobstructed action`).toBe(true);
}

export async function colorToken(page:Page, token:string) {
 return page.evaluate(token=>{
  const probe=document.createElement('span');
  probe.style.color='var('+token+')';
  document.body.append(probe);
  const value=getComputedStyle(probe).color;
  probe.remove();
  return value;
 },token);
}
