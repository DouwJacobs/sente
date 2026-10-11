import {test,expect} from '@playwright/test'
for(const {width,theme} of [{width:1440,theme:'light'},{width:900,theme:'dark'},{width:390,theme:'dark'},{width:360,theme:'light'}])test(`notification layout ${width} ${theme}`,async({page})=>{
 await page.setViewportSize({width,height:850});await page.addInitScript(t=>localStorage.setItem('finance-theme',t),theme)
 await page.route('**/api/notifications?*',route=>route.fulfill({json:{unread_count:2,total:3,items:[
 {id:901,title:'Groceries budget is nearly used',message:'Your Groceries budget has reached 85%. You have R 450,00 remaining for this period. Open the budget to review your categories and limits.',type:'budget_threshold',severity:'warning',source_kind:'budget',source_id:1,occurred_at:1791530400,created_at:1791530400,read_at:null,dismissible:1},
 {id:902,title:'A transaction needs your attention',message:'A bank description could not be matched to a category. Review the transaction before including it in your spending totals.',type:'system',severity:'info',source_kind:'transaction',source_id:2,occurred_at:1791526800,created_at:1791526800,read_at:null,dismissible:1},
 {id:903,title:'Your account import finished',message:'Your latest transactions are available. This deliberately long account notification checks that important details wrap within the card on a narrow screen.',type:'system',severity:'info',source_kind:'account',source_id:1,occurred_at:1791440400,created_at:1791440400,read_at:1791444000,dismissible:0}
 ]}}))
 await page.goto('/');await page.getByLabel('Username',{exact:true}).fill('demo');await page.getByLabel('Password',{exact:true}).fill('synthetic-browser-password');await page.getByRole('button',{name:'Sign in',exact:true}).click()
 await page.getByRole('button',{name:/^Notifications,/}).click();await expect(page.getByRole('region',{name:'Notification centre'})).toHaveAttribute('aria-busy','false');await expect(page.getByRole('article',{name:'Groceries budget is nearly used',exact:true})).toBeVisible()
 const centre=page.getByRole('region',{name:'Notification centre'})
 for(const control of await centre.getByRole('button').all()){const bounds=await control.boundingBox();expect(bounds!.height).toBeGreaterThanOrEqual(44);expect(bounds!.width).toBeGreaterThanOrEqual(44);expect(bounds!.x+bounds!.width).toBeLessThanOrEqual(width)}
 const actions=page.getByLabel('Notification actions',{exact:true});const box=await actions.boundingBox();expect(box!.height).toBeGreaterThanOrEqual(44)
 await expect(centre.getByRole('button',{name:'Mark all as read',exact:true})).toBeHidden();await actions.click();await expect(centre.getByRole('button',{name:'Mark all as read',exact:true})).toBeVisible();await page.keyboard.press('Escape');await expect(actions).toBeFocused()
 const alert=page.getByRole('article',{name:'Groceries budget is nearly used',exact:true});await expect(alert.getByText('Warning',{exact:true})).toBeVisible()
 await alert.getByLabel('Notification actions for Groceries budget is nearly used',{exact:true}).click();await expect(alert.getByRole('button',{name:'Dismiss',exact:true})).toBeVisible();await page.keyboard.press('Escape');await expect(alert.getByRole('button',{name:'Dismiss',exact:true})).toBeHidden()
 const read=page.getByRole('article',{name:'Your account import finished',exact:true});await expect(read.getByRole('button',{name:'Mark as read',exact:true})).toHaveCount(0);await expect(read.getByRole('button',{name:'Dismiss',exact:true})).toHaveCount(0)
 await page.screenshot({path:test.info().outputPath('notifications.png'),fullPage:true})
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
})
