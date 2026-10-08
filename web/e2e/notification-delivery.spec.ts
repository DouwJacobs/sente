import { test, expect, type Page } from "@playwright/test";
import { createECDH, randomBytes } from "node:crypto";
async function signIn(page: Page, url = "/") {
  await page.goto(url);
  await page.getByLabel("Username", { exact: true }).fill("demo");
  await page.getByLabel("Password", { exact: true }).fill("synthetic-browser-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByRole("button", { name: /^Notifications,/ })).toBeVisible();
}
async function preferences(page: Page) {
  await page.getByRole("button", { name: /^Notifications,/ }).click();
  await page.getByRole("button", { name: "Notification preferences", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Browser push devices" })).toBeVisible();
  await expect(page.getByLabel("System updates · Browser push", { exact: true })).toBeVisible();
}
test("Push opt-in stays explicit; channel drafts save together; device test and removal", async ({ page }) => {
  const ecdh = createECDH("prime256v1"); ecdh.generateKeys();
  const payload = { endpoint: "https://fcm.googleapis.com/fcm/send/synthetic-browser-workflow", keys: { p256dh: ecdh.getPublicKey().toString("base64url"), auth: randomBytes(16).toString("base64url") } };
  await page.addInitScript(({ payload }) => {
    let sub: any = null, registration: any = null;
    (window as any).pushPermissionRequests = 0;
    Object.defineProperty(Notification, "requestPermission", { value: async () => { (window as any).pushPermissionRequests++; return "granted"; } });
    Object.defineProperty(navigator, "serviceWorker", { value: {
      getRegistration: async () => registration,
      register: async () => {
        registration = { active: { scriptURL: location.origin + "/push-sw.js" }, pushManager: {
          getSubscription: async () => sub,
          subscribe: async (options: any) => {
            sub = { endpoint: payload.endpoint, options, toJSON: () => payload, unsubscribe: async () => { sub = null; return true; } }; return sub;
          },
        }};
        return registration;
      },
      get ready() { return Promise.resolve(registration); },
    }});
  }, { payload });
  await signIn(page); await preferences(page);
  await expect(page.getByLabel("System updates · Browser push", { exact: true })).toHaveValue("false");
  const system = page.locator(".notification-preference-row").getByText("System updates", { exact: true });
  await expect(system).toHaveCount(1);
  const row = page.getByRole("group", { name: "System updates", exact: true });
  await expect(row.getByRole("combobox")).toHaveCount(2);
  expect(await page.evaluate(() => (window as any).pushPermissionRequests)).toBe(0);
  await page.getByRole("button", { name: "Enable this browser", exact: true }).click();
  await expect(page.getByRole("button", { name: "This browser is enabled", exact: true })).toBeDisabled();
  expect(await page.evaluate(() => (window as any).pushPermissionRequests)).toBe(1);
  await page.getByLabel("System updates · In-app", { exact: true }).selectOption("false");
  await page.getByLabel("System updates · Browser push", { exact: true }).selectOption("true");
  await page.getByRole("tab", { name: "General", exact: true }).click();
  await page.getByRole("tab", { name: "Notifications", exact: true }).click();
  await expect(page.getByLabel("System updates · Browser push", { exact: true })).toHaveValue("true");
  await expect(page.getByRole("button", { name: "Save changes", exact: true })).toHaveCount(1);
  await page.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(page.getByText("Notification preferences saved", { exact: true })).toBeVisible();
  // Mock the test endpoint: no provider is contacted by browser fixtures.
  await page.route("**/api/notifications/push/subscriptions/*/test", route => route.fulfill({ json: { ok: true } }));
  await page.getByRole("button", { name: "Send test", exact: true }).click();
  await expect(page.getByText("Test notification queued. Check this device.", { exact: true })).toBeVisible();
  for (const width of [360, 390, 430, 760, 761, 1440]) {
    await page.setViewportSize({ width, height: 780 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
    const actions = page.locator(".notification-push-content > .toolbar-actions");
    expect(await actions.evaluate(element => parseFloat(getComputedStyle(element).gap))).toBeGreaterThanOrEqual(8);
    const controls = await actions.boundingBox();
    const devices = await page.locator(".notification-push-content > .notification-list").boundingBox();
    expect(devices!.y - (controls!.y + controls!.height)).toBeGreaterThanOrEqual(12);
  }
  await page.getByRole("button", { name: "Remove device", exact: true }).click();
  await expect(page.getByText("No push devices enabled.", { exact: true })).toBeVisible();
  expect(await page.evaluate(async () => (await navigator.serviceWorker.getRegistration("/"))?.pushManager.getSubscription())).toBeNull();
});
test("Denied permission and unsupported browsers keep in-app preferences usable", async ({ page }) => {
  await page.addInitScript(() => {
    Object.defineProperty(Notification, "requestPermission", { value: async () => "denied" });
  });
  await signIn(page); await preferences(page);
  await page.getByRole("button", { name: "Enable this browser", exact: true }).click();
  await expect(page.getByText("Browser notifications were not allowed. You can change this in browser settings.", { exact: true })).toBeVisible();
  await expect(page.getByText("No push devices enabled.", { exact: true })).toBeVisible();
  await page.addInitScript(() => { delete (window as any).PushManager; });
  await page.reload(); await preferences(page);
  await expect(page.getByRole("button", { name: "Enable this browser", exact: true })).toBeDisabled();
  await expect(page.getByText(/This browser does not support push here/)).toBeVisible();
  await expect(page.getByLabel("Budget thresholds · In-app", { exact: true })).toBeEnabled();
});
test("Push click link survives sign-in; administrator diagnostics filter and wrap", async ({ page }) => {
  await signIn(page, "/?notifications=1");
  await expect(page.getByRole("heading", { name: "Your notifications", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Notification preferences", exact: true }).click();
  await page.route("**/api/notifications/diagnostics?*", route => {
    const params = new URL(route.request().url()).searchParams;
    return route.fulfill({ json: { total: params.get("status") === "disabled" ? 0 : 1, items: params.get("status") === "disabled" ? [] : [{ id: 1, user_id: 2, username: "Synthetic recipient", type: "budget_threshold", key_hash: "a".repeat(64), evaluated_at: 1791450000, channel: "in_app", status: "deduplicated", error_code: "" }] } });
  });
  await page.getByRole("tab", { name: "Diagnostics", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Notification diagnostics", exact: true })).toBeVisible();
  await expect(page.getByText("Budget thresholds · deduplicated", { exact: true })).toBeVisible();
  await expect(page.getByRole("tablist", { name: "Settings sections" }).getByRole("tab", { name: "Notification diagnostics", exact: true })).toHaveCount(0);
  await page.getByRole("tab", { name: "Preferences", exact: true }).click();
  await page.getByLabel("System updates · Browser push", { exact: true }).selectOption("true");
  await page.getByRole("tab", { name: "Diagnostics", exact: true }).click();
  for (const [width, theme] of [[360, "light"], [390, "dark"], [430, "light"], [760, "dark"], [761, "light"], [1440, "dark"]] as const) {
    await page.setViewportSize({ width, height: 780 });
    await page.evaluate(value => document.documentElement.setAttribute("data-theme", value), theme);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
    const entry = await page.locator(".notification-diagnostic-row").boundingBox();
    expect(entry!.height).toBeLessThan(140);
  }
  await expect(page.getByText("Synthetic recipient · In-app", { exact: true })).toBeVisible();
  await expect(page.getByLabel("Logical event fingerprint", { exact: true })).toHaveText("a".repeat(64));
  await expect(page.locator(".notification-diagnostic-row details")).toHaveCount(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
  await page.getByRole("tab", { name: "Preferences", exact: true }).click();
  await expect(page.getByLabel("System updates · Browser push", { exact: true })).toHaveValue("true");
  await page.getByRole("tab", { name: "Diagnostics", exact: true }).click();
  await page.getByLabel("Delivery status", { exact: true }).selectOption("disabled");
  await expect(page.getByText("No diagnostics match this filter.", { exact: true })).toBeVisible();
});

test("Push-only message loads after sign-in and supports explicit read/dismiss", async ({ page }) => {
  let dismissed = false, read = false;
  await page.route("**/api/notifications/push/messages/99", route => route.fulfill(dismissed ? { status: 404, json: { error: "Notification unavailable" } } : { json: { id: 99, type: "system", severity: "info", title: "Push-only synthetic message", message: "This message is available through its authorized push link.", source_kind: "system", source_id: 0, occurred_at: 1791450000, created_at: 1791450000, read_at: read ? 1791450300 : null, dismissible: 1 } }));
  await page.route("**/api/notifications?*", route => route.fulfill({ json: { items: [], total: 0, inbox_total: 0, unread_count: 0 } }));
  await page.route("**/api/notifications/99/read", route => { read = true; return route.fulfill({ json: { ok: true, unread_count: 0 } }); });
  await page.route("**/api/notifications/99/dismiss", route => { dismissed = true; return route.fulfill({ json: { ok: true, unread_count: 0 } }); });
  await signIn(page, "/?notifications=1&message=99");
  const article = page.getByRole("article", { name: "Push-only synthetic message", exact: true });
  await expect(article).toBeVisible();
  await article.getByRole("button", { name: "Mark as read", exact: true }).click();
  await expect(article.getByText("Read", { exact: true })).toBeVisible();
  await article.getByRole("button", { name: "Dismiss", exact: true }).click();
  await expect(article).toHaveCount(0);
  expect(new URL(page.url()).searchParams.has("message")).toBeFalsy();
  await expect(page.getByText("Notification unavailable", { exact: true })).toHaveCount(0);
});


test("Push service registration failure explains recovery and allows retry without saving a device", async ({ page }) => {
  let saves = 0;
  await page.route("**/api/notifications/push/subscriptions", async route => {
    if (!route.request().postDataJSON().setup) saves++;
    await route.continue();
  });
  await page.addInitScript(() => {
    let registration: any = null;
    (window as any).subscribeAttempts = 0;
    Object.defineProperty(Notification, "requestPermission", { value: async () => "granted" });
    Object.defineProperty(navigator, "serviceWorker", { value: {
      getRegistration: async () => registration,
      register: async () => {
        registration = { active: { scriptURL: location.origin + "/push-sw.js" }, pushManager: {
          getSubscription: async () => null,
          subscribe: async () => {
            (window as any).subscribeAttempts++;
            throw new DOMException("Registration failed - push service error", "AbortError");
          },
        }};
        return registration;
      },
      get ready() { return Promise.resolve(registration); },
    }});
  });
  await signIn(page); await preferences(page);
  for (const width of [360, 1440]) {
    await page.setViewportSize({ width, height: 780 });
    const actions = page.locator(".notification-push-content > .toolbar-actions");
    expect(await actions.evaluate(element => parseFloat(getComputedStyle(element).gap))).toBeGreaterThanOrEqual(8);
    const controls = await actions.boundingBox();
    const empty = await page.getByText("No push devices enabled.", { exact: true }).boundingBox();
    expect(empty!.y - (controls!.y + controls!.height)).toBeGreaterThanOrEqual(12);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
  }
  const enable = page.getByRole("button", { name: "Enable this browser", exact: true });
  await enable.click();
  await expect(page.getByText(/This browser could not connect to its push service/)).toBeVisible();
  await expect(enable).toBeEnabled();
  await expect(page.getByText("No push devices enabled.", { exact: true })).toBeVisible();
  await enable.click();
  await expect.poll(() => page.evaluate(() => (window as any).subscribeAttempts)).toBe(2);
  expect(saves).toBe(0);
});
