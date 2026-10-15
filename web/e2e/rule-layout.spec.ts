import {desktopPhoneCases} from "./coverage-cases";
import {test,expect} from "@playwright/test";
for(const {width,theme} of desktopPhoneCases)test("unified rule indicators and keyboard actions "+width+" "+theme,async({page})=>{
 await page.setViewportSize({width,height:850});await page.addInitScript(t=>localStorage.setItem("finance-theme",t),theme);
 const pattern="Synthetic merchant description match with a long reference and service name";
 const rows=[
  {id:9001,account_id:1,account_name:"Everyday account",pattern,category_id:1,category_name:"Groceries",direction:"debit",priority:0,enabled:0,version:1,builtin:0,rule_kind:"category"},
  {id:-9002,account_id:0,account_name:"All accounts",pattern:"Global synthetic rule",category_id:1,category_name:"Groceries",direction:"any",priority:0,enabled:1,version:1,builtin:1,rule_kind:"category"},
  {id:9003,account_id:0,account_name:"All accounts",merchant_id:1,merchant_name:"Synthetic merchant with a long readable name",merchant_logo:"",pattern:"Merchant "+pattern,category_id:1,category_name:"Groceries",spending_group_name:"Recurring",direction:"credit",priority:2,enabled:1,version:1,builtin:0,rule_kind:"merchant"}
 ];
 await page.route("**/api/unified-rules?**",route=>route.fulfill({json:{items:rows.map(r=>({...r,rows:[r],refs:[{kind:r.rule_kind,id:r.id,version:r.version}],editable:true})),total:3,list_version:"synthetic"}}));
 await page.goto("/");await page.getByLabel("Username",{exact:true}).fill("demo");await page.getByLabel("Password",{exact:true}).fill("synthetic-browser-password");await page.getByRole("button",{name:"Sign in",exact:true}).click();
 if(width<760)await page.getByRole("button",{name:"More",exact:true}).click();await page.getByRole("button",{name:"Categories",exact:true}).click();await page.getByRole("tab",{name:"Rules",exact:true}).click();
 const row=page.locator(".rule-row:not(.merchant-rule-row)").filter({hasText:pattern});await expect(row.getByRole("img",{name:"Paused",exact:true})).toBeVisible();
 await expect(page.getByRole("img",{name:"Global fallback",exact:true})).toBeVisible();await row.locator("summary").click();await expect(row.getByRole("button",{name:"Resume",exact:true})).toBeVisible();await expect(row.getByRole("button",{name:"Apply to previous transactions",exact:true})).toBeDisabled();
 await page.keyboard.press("Escape");await expect(row.locator("summary")).toBeFocused();
 const merchant=page.locator(".merchant-rule-row");await expect(merchant).toContainText("Groceries");await expect(merchant).toContainText("Recurring");await expect(merchant).toContainText("All accounts");
 const avatar=(await merchant.locator(".merchant-avatar").boundingBox())!;expect(avatar.width).toBe(avatar.height);await merchant.locator("summary").click();await expect(merchant.getByRole("button",{name:"Apply to previous transactions",exact:true})).toBeVisible();
 await page.keyboard.press("Escape");expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});
