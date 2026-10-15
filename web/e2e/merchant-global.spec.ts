import {test,expect} from '@playwright/test'
const logo=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAEAAAABACAYAAACqaXHeAAAA50lEQVR4nO3TQQrDMAxE0Ryklyz0/u0qm4IbB8v8Ef2BWSWShgc5Hq/n+59z0AXoCEAXoCMAXYCOAHQBOgLQBegIQBegIwBdgI4AdAE6AtAF6JQAzDxJexGAu2V37MQBZgtX7xOAAhi9u7vr6kY7gFHxqj1RALPlK3bEAsz8CqtALQHOb1bnWwD8+m5lthXALMLqjWiAK4SqGwIkA4xmqm9EA3zP7bohQDLAObv7RjQAfUOA5HICCCCAAAIIIIAAAqQDdI4AdAE6AtAF6AhAF6AjAF2AjgB0AToC0AXoCEAXoCMAXYDOB+Arwn5fZL/mAAAAAElFTkSuQmCC','base64')
for(const width of [1440,360])test(`global merchants and optional logos ${width}`,async({page})=>{
 await page.setViewportSize({width,height:900});await page.goto('/');await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password');await page.getByRole('button',{name:'Sign in',exact:true}).click();await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible()
 const nav=async(name:string)=>{if(width===360&&!['Dashboard','Transactions','Review'].includes(name)){await page.getByRole('navigation',{name:'Mobile navigation'}).getByRole('button',{name:'More',exact:true}).click();await page.getByRole('navigation',{name:'More pages'}).getByRole('button',{name,exact:true}).click()}else await page.getByRole('navigation',{name:width===360?'Mobile navigation':'Main navigation',exact:true}).getByRole('button',{name,exact:true}).click()}
 await nav('Categories');await page.getByRole('tab',{name:'Merchant rules',exact:true}).click();await page.getByRole('button',{name:'Add merchant rule'}).click();const modal=page.getByRole('dialog',{name:'New merchant rule',exact:true});await expect(modal.getByLabel('Scope',{exact:true})).toHaveValue('0')
 const name='Logo merchant '+width;await modal.getByLabel('New merchant name',{exact:true}).fill(name);await modal.getByLabel('Description contains',{exact:true}).fill('Fuel');await modal.getByLabel('Merchant logo (optional)',{exact:true}).setInputFiles({name:'logo.png',mimeType:'image/png',buffer:logo});await expect(modal.getByRole('button',{name:'Remove logo'})).toBeVisible();await modal.getByRole('button',{name:'Save rule',exact:true}).click();await expect(modal).toBeHidden()
 const row=page.locator('.line').filter({hasText:name});await expect(row).not.toContainText('All accounts');await expect(row.locator('.merchant-avatar img')).toBeVisible();const avatarBox=(await row.locator('.merchant-avatar').boundingBox())!;expect(avatarBox.width).toBe(avatarBox.height);await expect(row.locator('.merchant-avatar')).toHaveCSS('border-radius','50%');await page.getByLabel('Actions for merchant rule '+name,{exact:true}).click();await page.getByRole('button',{name:'Edit',exact:true}).click();const edit=page.getByRole('dialog',{name:'Edit merchant rule',exact:true});await expect(edit.getByLabel('Scope',{exact:true})).toHaveValue('0');await expect(edit.locator('.merchant-avatar img')).toBeVisible();await edit.getByRole('button',{name:'Remove logo'}).click();await edit.getByRole('button',{name:'Save rule',exact:true}).click();await expect(edit).toBeHidden();await expect(row.locator('.merchant-avatar img')).toHaveCount(0)
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
 await nav('Transactions');await expect(page.locator('.merchant-avatar').first()).toBeVisible();expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
})

for (const width of [360,1440]) test(`one merchant with independent fee and interest rules ${width}`,async({page})=>{
 await page.setViewportSize({width,height:900});
 await page.addInitScript(t=>localStorage.setItem('finance-theme',t),width===360?'light':'dark');
 await page.goto('/');await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password');await page.getByRole('button',{name:'Sign in',exact:true}).click();await expect(page.getByRole('heading',{name:'Dashboard',exact:true})).toBeVisible();
 const me=await(await page.request.get('/api/me')).json();
 const write=async(path:string,body:any)=>{const response=await page.request.post('/api'+path,{data:body,headers:{'X-CSRF-Token':me.csrf,Origin:new URL(page.url()).origin}});expect(response.ok(),await response.text()).toBe(true);return response.json()};
 const merchant=await write('/labels',{kind:'merchant',name:`Synthetic FNB ${width}`,account_id:0});
 const categories=(await(await page.request.get('/api/categories?page=0')).json()).items;
 const groups=(await(await page.request.get('/api/spending-groups?page=0')).json()).items;
 const feeCategory=categories.find((c:any)=>c.kind==='expense');
 const interestCategory=categories.find((c:any)=>c.kind==='income');
 const first=await write('/merchant-rules',{merchant_id:merchant.id,pattern:`FNB account fee ${width}`,direction:'debit',enabled:true,category_id:feeCategory.id,spending_group_id:groups[0].id});
 await write('/merchant-rules',{merchant_id:merchant.id,pattern:`FNB interest paid ${width}`,direction:'credit',enabled:true,category_id:interestCategory.id,spending_group_id:groups[1].id});
 if(width===360){await page.getByRole('navigation',{name:'Mobile navigation'}).getByRole('button',{name:'More',exact:true}).click();await page.getByRole('navigation',{name:'More pages'}).getByRole('button',{name:'Categories',exact:true}).click()}else await page.getByRole('navigation',{name:'Main navigation'}).getByRole('button',{name:'Categories',exact:true}).click();
 await page.getByRole('tab',{name:'Merchant rules',exact:true}).click();
 const fees=page.locator('.merchant-rule-row').filter({hasText:`FNB account fee ${width}`});
 const interest=page.locator('.merchant-rule-row').filter({hasText:`FNB interest paid ${width}`});
 await expect(fees).toContainText(feeCategory.name);await expect(interest).toContainText(interestCategory.name);
 await fees.getByLabel(`Actions for merchant rule Synthetic FNB ${width}`,{exact:true}).click();await page.getByRole('button',{name:'Edit',exact:true}).click();
 const editor=page.getByRole('dialog',{name:'Edit merchant rule',exact:true});
 await expect(editor.getByRole('button',{name:new RegExp('^Category for this rule')})).toContainText(feeCategory.name);
 await expect(editor.getByRole('button',{name:new RegExp('^Spending group for this rule')})).toContainText(groups[0].name);
 await editor.getByLabel('Description contains',{exact:true}).fill(`FNB service fee ${width}`);
 await editor.getByRole('button',{name:'Save rule',exact:true}).click();await expect(editor).toBeHidden();
 await expect(interest).toContainText(interestCategory.name);await expect(interest).toContainText(groups[1].name);
 const saved=(await(await page.request.get('/api/merchant-rules?id='+first.id)).json()).items[0];
 expect(saved.category_id).toBe(feeCategory.id);expect(saved.spending_group_id).toBe(groups[0].id);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});
