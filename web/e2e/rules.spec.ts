import {test,expect,type Page} from "@playwright/test";
async function login(page:Page,width:number){
 await page.setViewportSize({width,height:900});await page.addInitScript(t=>localStorage.setItem("finance-theme",t),width===360?"light":"dark");
 await page.goto("/");await page.getByLabel("Username",{exact:true}).fill("demo");await page.getByLabel("Password",{exact:true}).fill("synthetic-browser-password");await page.getByRole("button",{name:"Sign in",exact:true}).click();
 await expect(page.getByRole("heading",{name:"Dashboard",exact:true})).toBeVisible();
 if(width===360){await page.getByRole("button",{name:"More",exact:true}).click();await page.getByRole("navigation",{name:"More pages"}).getByRole("button",{name:"Categories",exact:true}).click()}
 else await page.getByRole("navigation",{name:"Main navigation",exact:true}).getByRole("button",{name:"Categories",exact:true}).click();
 await page.getByRole("tab",{name:"Rules",exact:true}).click();
}
for(const width of [1440,360]){
 test("unified rules search, grouped edits and future-only creation "+width,async({page})=>{
  await login(page,width);const pattern="Unified EFT "+width;
  const before=await(await page.request.get("/api/transactions")).json();
  await page.getByRole("button",{name:"Add rule",exact:true}).click();const editor=page.getByRole("dialog",{name:"Add rule",exact:true});
  await editor.getByLabel("Description contains",{exact:true}).fill(pattern);
  await editor.getByRole("button",{name:"Save rule",exact:true}).click();
  await expect(editor.getByLabel("Category",{exact:true})).toHaveAttribute("aria-invalid","true");
  await expect(editor.getByLabel("Category",{exact:true})).toBeFocused();
  await editor.getByLabel("Category",{exact:true}).click();await page.getByRole("dialog",{name:"Select category",exact:true}).getByRole("button").filter({hasText:"Groceries"}).first().click();
  await editor.getByText("Check an example",{exact:true}).click();await editor.getByLabel("Example description",{exact:true}).fill(pattern+" reference");
  await editor.getByRole("button",{name:"Check rule",exact:true}).click();await expect(editor.getByRole("status")).toContainText("Groceries");
  await editor.getByRole("button",{name:"Save rule",exact:true}).click();const offered=page.getByRole("dialog",{name:"Apply this rule to previous transactions?",exact:true});
  await expect(offered).toBeVisible();await offered.getByRole("button",{name:"Not now",exact:true}).click();
  expect(await(await page.request.get("/api/transactions")).json()).toEqual(before);
  await page.getByLabel("Search rules",{exact:true}).fill(pattern);const row=page.locator(".rule-row").filter({hasText:pattern});await expect(row).toHaveCount(1);
  await row.locator("summary").click();await row.getByRole("button",{name:"Pause",exact:true}).click();await expect(row.getByRole("img",{name:"Paused",exact:true})).toBeVisible();
  await row.locator("summary").click();await expect(row.getByRole("button",{name:"Apply to previous transactions",exact:true})).toBeDisabled();
  await row.getByRole("button",{name:"Resume",exact:true}).click();await expect(row.getByRole("img",{name:"Active",exact:true})).toBeVisible();
  await row.locator("summary").click();await row.getByRole("button",{name:"Edit",exact:true}).click();
  const edit=page.getByRole("dialog",{name:"Edit rule",exact:true});await edit.getByLabel("Description contains",{exact:true}).fill(pattern+" updated");await edit.getByRole("button",{name:"Save rule",exact:true}).click();await expect(edit).toBeHidden();
  await expect(row).toContainText("updated");await page.getByLabel("Search rules",{exact:true}).fill("no such rule");await expect(page.getByRole("heading",{name:"No rules found",exact:true})).toBeVisible();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 });
 test("group-only rule and explicit manual backfill "+width,async({page})=>{
  await login(page,width);const pattern="Group-only "+width;await page.getByRole("button",{name:"Add rule",exact:true}).click();
  const editor=page.getByRole("dialog",{name:"Add rule",exact:true});await editor.getByLabel("Description contains",{exact:true}).fill(pattern);
  await editor.getByLabel("Spending group",{exact:true}).click();await page.getByRole("dialog",{name:"Select spending group",exact:true}).getByRole("button",{name:/^Recurring(?: Selected)?$/}).click();
  await editor.getByRole("button",{name:"Save rule",exact:true}).click();await page.getByRole("dialog",{name:"Apply this rule to previous transactions?",exact:true}).getByRole("button",{name:"Not now",exact:true}).click();
  await page.getByLabel("Search rules",{exact:true}).fill(pattern);const row=page.locator(".rule-row").filter({hasText:pattern});await expect(row).toContainText("Recurring");
  await row.locator("summary").click();await row.getByRole("button",{name:"Apply to previous transactions",exact:true}).click();
  const backfill=page.getByRole("dialog",{name:"Apply to previous transactions",exact:true});await backfill.getByLabel("Previous transactions",{exact:true}).selectOption("period");
  await backfill.getByLabel("Budget period",{exact:true}).selectOption("1");await backfill.getByRole("button",{name:"Preview changes",exact:true}).click();
  await expect(backfill.getByRole("heading",{name:"No changes to apply",exact:true})).toBeVisible();await backfill.getByRole("button",{name:"Cancel",exact:true}).click();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 });
}

for(const {width,height} of [{width:430,height:800},{width:760,height:400},{width:761,height:400}])for(const theme of ["light","dark"])test("rule editor and backfill fit "+width+"x"+height+" "+theme,async({page})=>{
 await login(page,1440);await page.setViewportSize({width,height});await page.evaluate(t=>{localStorage.setItem("finance-theme",t);document.documentElement.dataset.theme=t},theme);
 await page.getByRole("button",{name:"Add rule",exact:true}).click();const editor=page.getByRole("dialog",{name:"Add rule",exact:true});
 await editor.getByLabel("Description contains",{exact:true}).fill("Synthetic long description for narrow and short landscape rule controls "+width+" "+theme);
 await editor.getByLabel("Category",{exact:true}).click();await page.getByRole("dialog",{name:"Select category",exact:true}).getByRole("button").filter({hasText:"Groceries"}).first().click();
 await editor.getByRole("button",{name:"Save rule",exact:true}).click();const offer=page.getByRole("dialog",{name:"Apply this rule to previous transactions?",exact:true});await expect(offer).toBeVisible();
 await offer.getByRole("button",{name:"Preview changes",exact:true}).click();await expect(offer.getByRole("heading",{name:"No changes to apply",exact:true})).toBeVisible();
 await offer.getByRole("button",{name:"Not now",exact:true}).scrollIntoViewIfNeeded();await expect(offer.getByRole("button",{name:"Not now",exact:true})).toBeVisible();
 const bounds=(await offer.boundingBox())!;expect(bounds.width).toBeLessThanOrEqual(width);expect(bounds.height).toBeLessThanOrEqual(height);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 await offer.getByRole("button",{name:"Not now",exact:true}).click();
});
