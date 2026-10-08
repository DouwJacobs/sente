import { test, expect, type Page } from "@playwright/test";
async function signIn(page: Page) {
  await page.goto("/");
  await page.getByLabel("Username", { exact: true }).fill("demo");
  await page.getByLabel("Password", { exact: true }).fill("synthetic-browser-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Dashboard", exact: true })).toBeVisible();
}
async function openInbox(page: Page) {
  await page.getByRole("button", { name: /^Notifications,/ }).click();
  await expect(page.getByRole("heading", { name: "Your notifications" })).toBeVisible();
  await expect(page.getByRole("region", { name: "Notification centre" })).toHaveAttribute("aria-busy", "false");
}
test("Persistent inbox, authorized links, preferences and responsive keyboard workflow", async ({ page }) => {
  await signIn(page);
  await expect(page.getByRole("button", { name: "Notifications, 26 unread", exact: true })).toBeVisible();
  await openInbox(page);
  await expect(page.getByRole("article", { name: "System message cannot dismiss" }).getByRole("button", { name: "Dismiss" })).toHaveCount(0);
  await expect(page.getByText("Another user's private message")).toHaveCount(0);
  await expect(page.getByText("Inaccessible private details")).toHaveCount(0);
  const transaction = page.getByRole("article", { name: "Transaction message", exact: true });
  await transaction.getByRole("button", { name: "View transaction" }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(transaction.getByRole("button", { name: "View transaction" })).toBeFocused();
  await transaction.getByRole("button", { name: "Mark as read", exact: true }).click();
  await expect(page.getByRole("button", { name: "Notifications, 25 unread", exact: true })).toBeVisible();
  await expect(transaction.getByText("Read", { exact: true })).toBeVisible();
  await transaction.getByRole("button", { name: "Dismiss", exact: true }).click();
  await expect(transaction).toHaveCount(0);
  await page.getByRole("button", { name: "Next notifications" }).click();
  await expect(page.getByText("Page 2 of 2")).toBeVisible();
  await page.getByRole("button", { name: "Previous notifications" }).click();
  await page.getByRole("article", { name: "Account message", exact: true }).getByRole("button", { name: "View account" }).click();
  await expect(page.getByRole("heading", { name: "Transactions", exact: true })).toBeVisible();
  await openInbox(page);
  await page.getByRole("article", { name: "Budget message", exact: true }).getByRole("button", { name: "View budget" }).click();
  await expect(page.getByRole("heading", { name: "Budgets", exact: true })).toBeVisible();
  await openInbox(page);
  for (const [width, height, theme] of [[1440,900,"light"], [360,780,"dark"], [390,780,"light"], [430,780,"dark"], [760,430,"light"], [761,900,"dark"]] as const) {
    await page.setViewportSize({ width, height });
    await page.evaluate(value => document.documentElement.setAttribute("data-theme", value), theme);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
    const bell = await page.getByRole("button", { name: /^Notifications,/ }).boundingBox();
    expect(bell!.width).toBeGreaterThanOrEqual(44);
    expect(bell!.height).toBeGreaterThanOrEqual(44);
  }
  await page.setViewportSize({ width:360, height:780 });
  await page.getByRole("tab", { name:"All", exact:true }).focus();
  await page.keyboard.press("End");
  await expect(page.getByRole("tab", { name:"Read", exact:true })).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("heading", { name:"No notifications", exact:true })).toBeVisible();
  await page.getByRole("button", { name:"Notification preferences", exact:true }).click();
  const field = page.getByLabel("Budget thresholds · In-app", { exact:true });
  await field.selectOption("false");
  await page.getByLabel("Projected overspend · In-app", { exact:true }).selectOption("false");
  await expect(page.getByRole("button", { name:"Save changes", exact:true })).toHaveCount(1);
  // Mounted preference drafts survive tab changes.
  await page.getByRole("tab", { name:"General", exact:true }).click();
  await page.getByRole("tab", { name:"Notifications", exact:true }).click();
  await expect(field).toHaveValue("false");
  await field.locator("xpath=ancestor::form").getByRole("button", { name:"Save changes" }).click();
  await expect(page.getByText("Notification preferences saved", { exact:true })).toBeVisible();
  await page.reload();
  await openInbox(page);
  await page.getByRole("button", { name:"Notification preferences", exact:true }).click();
  await expect(field).toHaveValue("false");
  await expect(page.getByLabel("Projected overspend · In-app", { exact:true })).toHaveValue("false");
  for (const width of [360, 1440]) {
    await page.setViewportSize({ width, height:780 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
    for (const control of await page.locator(".notification-preferences select").all()) {
      const box = await control.boundingBox();
      expect(box!.height).toBeGreaterThanOrEqual(44);
      expect(box!.x + box!.width).toBeLessThanOrEqual(width);
    }
  }
  await page.setViewportSize({ width:360, height:780 });
  await openInbox(page);
  await page.getByRole("button", { name:"Mark all as read", exact:true }).click();
  await expect(page.getByRole("button", { name:"Notifications, 0 unread", exact:true })).toBeVisible();
  await page.getByRole("tab", { name:"Unread", exact:true }).click();
  await expect(page.getByRole("heading", { name:"No notifications", exact:true })).toBeVisible();
});
test("List failure retries and unavailable target fails safely", async ({ page }) => {
  await signIn(page);
  await page.route("**/api/notifications?*", route => route.fulfill({ status:503, json:{ error:"Synthetic inbox outage" } }));
  await openInbox(page);
  await expect(page.getByRole("heading", { name:"Notifications could not be loaded" })).toBeVisible();
  await page.unroute("**/api/notifications?*");
  await page.getByRole("button", { name:"Retry notifications" }).click();
  const account = page.getByRole("article", { name:"Account message", exact:true });
  await expect(account).toBeVisible();
  await page.route("**/api/accounts?page=0&page_size=1&id=1", route => route.fulfill({ json:{ items:[] } }));
  await account.getByRole("button", { name:"View account" }).click();
  await expect(page.getByText("This notification target is no longer available.", { exact:true })).toBeVisible();
  await expect(page.getByRole("heading", { name:"Your notifications" })).toBeVisible();
});
test("Preference stale-write keeps draft and explicit reload recovers", async ({ page }) => {
  await signIn(page);
  await openInbox(page);
  await page.getByRole("button", { name:"Notification preferences", exact:true }).click();
  const field = page.getByLabel("System updates · In-app", { exact:true });
  await field.selectOption("false");
  await page.route("**/api/notifications/preferences/batch*", async route => {
    if (route.request().method() === "PUT") await route.fulfill({ status:409, json:{ error:"Notification preference changed; reload before saving" } });
    else await route.continue();
  });
  await field.locator("xpath=ancestor::form").getByRole("button", { name:"Save changes" }).click();
  await expect(page.getByText("Notification preference changed; reload before saving", { exact:true })).toBeVisible();
  await expect(field).toHaveValue("false");
  await page.getByRole("button", { name:"Reload saved preferences" }).click();
  await expect(field).toHaveValue("true");
});
