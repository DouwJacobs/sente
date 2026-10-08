import { test, expect } from "@playwright/test";

test("Dashboard budget editor saves budget and personal alerts together", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("Username", { exact: true }).fill("demo");
  await page.getByLabel("Password", { exact: true }).fill("synthetic-browser-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Dashboard", exact: true })).toBeVisible();
  const session = await (await page.request.get("/api/me")).json();
  const periods = await (await page.request.get("/api/periods?id=1")).json();
  const groups = await (await page.request.get("/api/spending-groups")).json();
  const exceptions = groups.find((group: any) => group.name === "Exceptions");
  expect((await page.request.put("/api/periods/1/budget", {
    headers: { "X-CSRF-Token": session.csrf, Origin: new URL(page.url()).origin },
    data: { version: periods.items[0].version, groups: [{ group_id: exceptions.id, targets: [{ category_id: 1, amount_cents: 100000, carry_forward: false }] }] },
  })).ok()).toBe(true);
  await page.reload();
  const bucket = (name: string) => page.locator(".spending-bucket").filter({ has: page.locator(".bucket-name strong", { hasText: name }) });
  const open = async (group: string) => {
    const current = bucket(group);
    if ((await current.getAttribute("open")) === null) await current.locator("summary").first().click();
    await current.getByRole("button", { name: "Groceries transactions", exact: true }).click();
    const summary = page.getByRole("dialog", { name: `Groceries · ${group}`, exact: true });
    await expect(summary.getByLabel("Alert threshold (%)", { exact: true })).toHaveCount(0);
    await summary.getByRole("button", { name: "Edit budget for Groceries", exact: true }).click();
    const editor = page.getByRole("dialog", { name: "Edit budget · Groceries", exact: true });
    await expect(editor.getByLabel("Alert threshold (%)", { exact: true })).toBeEnabled();
    return { editor, summary };
  };
  let { editor, summary } = await open("No spending group");
  await expect(editor.getByRole("button", { name: /^Save/ })).toHaveCount(1);
  const threshold = editor.getByLabel("Alert threshold (%)", { exact: true });
  const reset = editor.getByRole("button", { name: "Reset alert to defaults", exact: true });
  await expect(reset).toBeVisible(); await expect(reset).toBeDisabled();
  const resetBounds = await reset.boundingBox();
  const bellBounds = await editor.getByRole("button", { name: "Mute budget alerts", exact: true }).boundingBox();
  expect(resetBounds!.x + resetBounds!.width).toBeLessThanOrEqual(bellBounds!.x);
  const beforeEditing = await editor.getByRole("button", { name: "Save changes", exact: true }).boundingBox();
  await threshold.fill("70");
  expect((await editor.getByRole("button", { name: "Save changes", exact: true }).boundingBox())!.y).toBe(beforeEditing!.y);
  await threshold.fill("101"); await threshold.blur();
  await expect(threshold).toHaveAttribute("aria-invalid", "true");
  await threshold.fill("70"); await expect(threshold).not.toHaveAttribute("aria-invalid", "true");
  await editor.getByLabel("Budget amount", { exact: true }).fill("6000.00");
  await editor.getByRole("button", { name: "Mute budget alerts", exact: true }).click();
  await expect(editor.getByRole("button", { name: "Enable budget alerts", exact: true })).toHaveAttribute("aria-pressed", "false");
  let snapshot = (await (await page.request.get("/api/notifications/preferences?channels=all")).json()).budget_items;
  expect(snapshot.find((item: any) => item.category_id === 1 && item.group_id === 0)).toMatchObject({ enabled: true, threshold: null });
  for (const [width, height, theme] of [[1440, 900, "light"], [360, 780, "dark"], [760, 430, "dark"]] as const) {
    await page.setViewportSize({ width, height });
    await page.evaluate(value => document.documentElement.dataset.theme = value, theme);
    await threshold.scrollIntoViewIfNeeded();
    expect(await editor.evaluate(element => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  }
  await editor.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(editor).toHaveCount(0);
  snapshot = (await (await page.request.get("/api/notifications/preferences?channels=all")).json()).budget_items;
  expect(snapshot.find((item: any) => item.category_id === 1 && item.group_id === 0)).toMatchObject({ enabled: false, threshold: 70 });
  const limits = await (await page.request.get("/api/periods/1/targets?group=0&budget_only=1&id=1")).json();
  expect(limits.items[0].amount_cents).toBe(600000);
  await summary.getByRole("button", { name: "Close", exact: true }).click();
  ({ editor, summary } = await open("Exceptions"));
  await editor.getByLabel("Alert threshold (%)", { exact: true }).fill("90");
  await editor.getByRole("button", { name: "Save changes", exact: true }).click(); await expect(editor).toHaveCount(0);
  await summary.getByRole("button", { name: "Close", exact: true }).click();
  ({ editor, summary } = await open("Exceptions"));
  await expect(editor.getByLabel("Alert threshold (%)", { exact: true })).toHaveValue("90");
  await editor.getByLabel("Alert threshold (%)", { exact: true }).fill("80");
  await editor.getByLabel("Budget amount", { exact: true }).fill("1500.00");
  await page.route("**/api/periods/1/budget", route => route.fulfill({ status: 409, json: { error: "Budget alert settings changed; reload before saving" } }));
  await editor.getByRole("button", { name: "Save changes", exact: true }).click();
  await expect(page.getByText("Budget alert settings changed; reload before saving", { exact: true })).toBeVisible();
  await expect(editor.getByLabel("Alert threshold (%)", { exact: true })).toHaveValue("80");
  await expect(editor.getByLabel("Budget amount", { exact: true })).toHaveValue("1500.00");
  await page.unroute("**/api/periods/1/budget");
  await editor.getByRole("button", { name: "Reload alert", exact: true }).click();
  await expect(editor.getByLabel("Alert threshold (%)", { exact: true })).toHaveValue("90");
  await editor.getByRole("button", { name: "Reset alert to defaults", exact: true }).click();
  await expect(editor.getByLabel("Alert threshold (%)", { exact: true })).toHaveValue("");
  await editor.getByRole("button", { name: "Save changes", exact: true }).click(); await expect(editor).toHaveCount(0);
  await summary.getByRole("button", { name: "Close", exact: true }).click();
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.getByRole("navigation", { name: "Main navigation", exact: true }).getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByRole("tab", { name: "Notifications", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Notification preferences", exact: true })).toBeVisible();
  await expect(page.getByLabel("Alert threshold (%)", { exact: true })).toHaveCount(0);
});
