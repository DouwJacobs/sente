
import { test, expect, type Page, type Locator } from "@playwright/test";

async function login(page: Page, width: number) {
  await page.setViewportSize({ width, height: 900 });
  await page.addInitScript(theme => localStorage.setItem("finance-theme", theme), width === 360 ? "light" : "dark");
  await page.goto("/");
  await page.getByLabel("Username", { exact: true }).fill("demo");
  await page.getByLabel("Password", { exact: true }).fill("synthetic-browser-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Dashboard", exact: true })).toBeVisible();
}
async function write(page: Page, path: string, body: unknown, method = "POST") {
  const me = await (await page.request.get("/api/me")).json();
  const response = await page.request.fetch("/api" + path, { method, data: body, headers: { "X-CSRF-Token": me.csrf } });
  expect(response.ok(), await response.text()).toBe(true);
  return response.json();
}
async function transaction(page: Page, id = 5) {
  return (await (await page.request.get("/api/transactions?id=" + id)).json()).items[0];
}
async function reset(page: Page) {
  const row = await transaction(page);
  await write(page, "/transactions/5", {
    version: row.version, date: row.date, description: row.description,
    amount_cents: row.amount_cents, is_transfer: false, assignment: "auto",
    merchant_id: null, clear_merchant: true, spending_group_id: null, note: "",
    allocations: [{ category_id: null, amount_cents: row.amount_cents }],
  }, "PUT");
  return row;
}
async function choose(page: Page, editor: Locator, label: string, name: string) {
  await editor.getByLabel(label, { exact: true }).click();
  const picker = page.getByRole("dialog", { name: "Select " + label.toLowerCase().replace(" for this rule", ""), exact: true });
  await picker.getByRole("button", { name: new RegExp("^" + name + "(?: |$)") }).first().click();
}
for (const width of [360, 1440]) {
  test("merchant defaults follow corrections and preserve explicit choices " + width, async ({ page }) => {
    await login(page, width);
    const row = await reset(page);
    const categories = (await (await page.request.get("/api/categories?page=0")).json()).items;
    const groups = (await (await page.request.get("/api/spending-groups?page=0")).json()).items;
    const groceries = categories.find((c: any) => c.name === "Groceries");
    const other = categories.find((c: any) => c.kind === "expense" && c.id !== groceries.id);
    const recurring = groups.find((g: any) => g.name === "Recurring");
    const exceptions = groups.find((g: any) => g.name === "Exceptions");
    for (const [name, category, group] of [
      ["Default Alpha " + width, groceries.id, recurring.id],
      ["Default Beta " + width, other.id, exceptions.id],
      ["Default Group " + width, null, recurring.id],
    ]) await write(page, "/labels", { kind: "merchant", name, account_id: 0, category_id: category, spending_group_id: group });
    await page.getByRole("navigation", { name: width === 360 ? "Mobile navigation" : "Main navigation", exact: true }).getByRole("button", { name: "Transactions", exact: true }).click();
    await page.locator(".transaction-detail").filter({ hasText: row.description }).click();
    const editor = page.getByRole("dialog", { name: "Transaction #5", exact: true });
    await editor.getByText("Notes, merchant and tags", { exact: true }).click();
    await choose(page, editor, "Merchant", "Default Alpha " + width);
    await expect(editor.getByLabel("Category", { exact: true })).toContainText("Groceries");
    await choose(page, editor, "Merchant", "Default Beta " + width);
    await expect(editor.getByLabel("Category", { exact: true })).toContainText(other.name);
    await expect(editor.getByLabel("Spending group", { exact: true })).toContainText("Exceptions");
    await choose(page, editor, "Merchant", "Not set");
    await expect(editor.getByLabel("Category", { exact: true })).toContainText("Uncategorized");
    await expect(editor.getByLabel("Spending group", { exact: true })).toContainText("Not set");
    await choose(page, editor, "Merchant", "Default Group " + width);
    await expect(editor.getByLabel("Category", { exact: true })).toContainText("Uncategorized");
    await expect(editor.getByLabel("Spending group", { exact: true })).toContainText("Recurring");
    await choose(page, editor, "Category", "Groceries");
    await choose(page, editor, "Spending group", "Not set");
    await choose(page, editor, "Merchant", "Default Beta " + width);
    await expect(editor.getByLabel("Category", { exact: true })).toContainText("Groceries");
    await expect(editor.getByLabel("Spending group", { exact: true })).toContainText("Not set");
    await editor.getByRole("button", { name: "Save changes", exact: true }).click();
    await expect(editor).toBeHidden();
    const saved = await transaction(page);
    expect(saved.allocations[0].category_id).toBe(groceries.id);
    expect(saved.spending_group_id).toBeNull();
    expect(saved.review_state).toBe("approved");
    await page.locator(".transaction-detail").filter({ hasText: row.description }).click();
    await choose(page, editor, "Merchant", "Default Alpha " + width);
    await expect(editor.getByLabel("Category", { exact: true })).toContainText("Groceries");
    await expect(editor.getByLabel("Spending group", { exact: true })).toContainText("Recurring");
    await choose(page, editor, "Category", "Uncategorized");
    await choose(page, editor, "Merchant", "Default Beta " + width);
    await expect(editor.getByLabel("Category", { exact: true })).toContainText("Uncategorized");
    await editor.getByRole("button", { name: "Add split", exact: true }).click();
    await editor.getByLabel("Category 1", { exact: true }).click();
    await page.getByRole("dialog", { name: "Select category 1", exact: true }).getByRole("button", { name: /^Groceries(?: |$)/ }).first().click();
    const splitGroup = await editor.getByLabel("Spending group 1", { exact: true }).textContent();
    await choose(page, editor, "Merchant", "Default Alpha " + width);
    await expect(editor.getByLabel("Category 1", { exact: true })).toContainText("Groceries");
    expect(await editor.getByLabel("Spending group 1", { exact: true }).textContent()).toBe(splitGroup);
    await editor.locator(".transaction-editor-footer").getByRole("button", { name: "Close", exact: true }).click();
    await page.getByRole("dialog", { name: "Unsaved changes", exact: true }).getByRole("button", { name: "Discard changes", exact: true }).click();

  });

  test("review retry counts remaining entries without another save " + width, async ({ page }) => {
    await login(page, width);
    const row = await reset(page);
    await page.getByRole("navigation", { name: width === 360 ? "Mobile navigation" : "Main navigation", exact: true }).getByRole("button", { name: "Transactions", exact: true }).click();
    await page.getByRole("tab", { name: /Needs review/ }).click();
    await expect(page.getByText("Loading transactions", { exact: true })).toBeHidden();
    await page.locator(".transaction-detail").filter({ hasText: row.description }).click();
    const editor = page.getByRole("dialog", { name: "Transaction #5", exact: true });
    const before = await transaction(page);
    let fail = true;
    let checkingQueue = false;
    await page.route("**/api/transactions/navigation?**", async route => {
      const response = await route.fetch();
      const result = await response.json();
      if (!result.next && !result.previous) checkingQueue = true;
      await route.fulfill({ response, json: result });
    });
    await page.route("**/api/transactions?**", async route => {
      const url = new URL(route.request().url());
      if (checkingQueue && url.searchParams.get("offset") === "0" && url.searchParams.get("acceptance") === "needs_category" && fail) {
        await route.fulfill({ status: 503, json: { error: "Synthetic count unavailable" } });
      } else await route.fallback();
    });
    await editor.getByRole("button", { name: "Save and next", exact: true }).click();
    const unavailable = page.getByRole("dialog", { name: "Review status unavailable", exact: true });
    await expect(unavailable).toBeVisible();
    fail = false;
    await unavailable.getByRole("button", { name: "Retry", exact: true }).click();
    const end = page.getByRole("dialog", { name: "End of this pass", exact: true });
    await expect(end).toContainText("1 transactions still need categories");
    expect((await transaction(page)).version).toBe(before.version + 1);
    await end.getByRole("button", { name: "Review remaining", exact: true }).click();
    await expect(editor).toBeVisible();
    await choose(page, editor, "Category", "Groceries");
    await editor.getByRole("button", { name: "Save and next", exact: true }).click();
    await expect(page.getByRole("dialog", { name: "Review complete", exact: true })).toBeVisible();
    expect((await transaction(page)).review_state).toBe("approved");
  });
}

for (const width of [360, 1440]) test("transfer actions guard drafts and recover after a failed link " + width, async ({ page }) => {
  await login(page, width);
  const source = await reset(page);
  const me = await (await page.request.get("/api/me")).json();
  const description = "Synthetic transfer counterpart " + width;
  const csv = "ACCOUNT TRANSACTION HISTORY\n\nName:,Synthetic\nAccount:,22222222222,Savings\nBalance:,100.00\n\nDate, Amount, Balance, Description\n2026/10/25,1250.00,100.00," + description + "\n";
  const previewResponse = await page.request.post("/api/imports/preview?paged=1", {
    headers: { "X-CSRF-Token": me.csrf },
    multipart: { account_id: "2", files: { name: "transfer.csv", mimeType: "text/csv", buffer: Buffer.from(csv) } },
  });
  expect(previewResponse.ok(), await previewResponse.text()).toBe(true);
  const reports = await previewResponse.json();
  const preview = reports[0];
  await write(page, "/imports/" + preview.id + "/commit", { classification_version: preview.classification_version, preview_version: preview.preview_version, decisions: {}, confirm_valid_rows: false, skip_all_candidates: false });
  const counterpart = (await (await page.request.get("/api/transactions?q=" + encodeURIComponent(description))).json()).items[0];
  await page.getByRole("navigation", { name: width === 360 ? "Mobile navigation" : "Main navigation", exact: true }).getByRole("button", { name: "Transactions", exact: true }).click();
  await page.locator(".transaction-detail").filter({ hasText: source.description }).click();
  const editor = page.getByRole("dialog", { name: "Transaction #5", exact: true });
  await editor.getByText("Notes, merchant and tags", { exact: true }).click();
  await editor.getByLabel("Transaction note", { exact: true }).fill("Preserve transfer draft " + width);
  await choose(page, editor, "Category", "Groceries");
  await editor.getByText("Automatically categorize similar transactions", { exact: true }).click();
  await editor.getByLabel("Use this category and spending group for similar transactions in this account", { exact: true }).check();
  const pattern = "Transfer continuation " + width;
  await editor.getByLabel("Description contains", { exact: true }).fill(pattern);

  await editor.getByText("Transfer links, original details and history", { exact: true }).click();
  await editor.getByText("Link or unlink transfer", { exact: true }).click();
  await editor.getByLabel("Transaction number in the other account", { exact: true }).fill(String(counterpart.id));
  await editor.getByRole("button", { name: "Find transaction", exact: true }).click();
  await editor.getByRole("button", { name: "Link transactions", exact: true }).click();
  const guard = page.getByRole("dialog", { name: "Unsaved changes", exact: true });
  await guard.getByRole("button", { name: "Stay here", exact: true }).click();
  await expect(editor.getByLabel("Transaction note", { exact: true })).toHaveValue("Preserve transfer draft " + width);
  expect((await transaction(page)).transfer_counterpart_id).toBeFalsy();
  const beforeInvalidSave = await transaction(page);
  await editor.getByLabel("Date", { exact: true }).fill("");
  await editor.getByRole("button", { name: "Link transactions", exact: true }).click();
  await guard.getByRole("button", { name: "Save and continue", exact: true }).click();
  await expect(editor.getByLabel("Date", { exact: true })).toHaveAttribute("aria-invalid", "true");
  await expect(editor.getByLabel("Date", { exact: true })).toBeFocused();
  expect((await transaction(page)).version).toBe(beforeInvalidSave.version);
  await editor.getByLabel("Date", { exact: true }).fill(source.date);
  let reject = true;
  let transferCalls = 0;
  await page.route("**/api/transfers", async route => {
    transferCalls++;
    if (reject) {
      reject = false;
      await route.fulfill({ status: 409, json: { error: "Synthetic transfer conflict" } });
    } else await route.continue();
  });
  await editor.getByRole("button", { name: "Link transactions", exact: true }).click();
  await guard.getByRole("button", { name: "Save and continue", exact: true }).click();
  const offer = page.getByRole("dialog", { name: "Apply this rule to previous transactions?", exact: true });
  await expect(offer).toBeVisible();
  expect(transferCalls).toBe(0);
  await offer.getByRole("button", { name: "Not now", exact: true }).click();
  await expect(page.locator(".toast:popover-open")).toContainText("Synthetic transfer conflict");
  await expect(editor).toBeVisible();
  const saved = await transaction(page);
  expect(saved.note).toBe("Preserve transfer draft " + width);
  expect(saved.allocations[0].category_id).toBe(1);
  expect(transferCalls).toBe(1);
  if (!await editor.getByRole("button", { name: "Link transactions", exact: true }).isVisible()) {
    await editor.getByText("Transfer links, original details and history", { exact: true }).click();
    if (!await editor.getByRole("button", { name: "Link transactions", exact: true }).isVisible()) {
      await editor.getByText("Link or unlink transfer", { exact: true }).click();
    }
  }
  await editor.getByRole("button", { name: "Link transactions", exact: true }).evaluate(button => {
    (button as HTMLButtonElement).click();
    (button as HTMLButtonElement).click();
  });
  await expect(editor).toBeHidden();
  expect(transferCalls).toBe(2);
  expect((await (await page.request.get("/api/unified-rules?page=0&q=" + encodeURIComponent(pattern))).json()).total).toBe(1);
  const linked = await transaction(page);
  expect(linked.transfer_counterpart_id).toBe(counterpart.id);
  expect(linked.version).toBe(saved.version + 1);
  expect((await transaction(page, counterpart.id)).transfer_counterpart_id).toBe(5);
  await page.locator(".transaction-detail").filter({ hasText: source.description }).click();
  await editor.getByLabel("Transaction note", { exact: true }).fill("Discard this transfer draft");
  await editor.getByText("Transfer links, original details and history", { exact: true }).click();
  await editor.getByText("Link or unlink transfer", { exact: true }).click();
  await editor.getByRole("button", { name: "Unlink transfer", exact: true }).click();
  await guard.getByRole("button", { name: "Discard changes", exact: true }).click();
  await expect(editor).toBeHidden();
  const unlinked = await transaction(page);
  expect(unlinked.transfer_counterpart_id).toBeFalsy();
  expect(unlinked.note).toBe("Preserve transfer draft " + width);
  expect((await transaction(page, counterpart.id)).transfer_counterpart_id).toBeFalsy();
});

for (const width of [360, 1440]) test("draft merchant example rolls back and date range preview resets " + width, async ({ page }) => {
  await login(page, width);
  const target = await reset(page);
  const pattern = "Draft transaction " + width;
  const current = await transaction(page);
  await write(page, "/transactions/5", {
    version: current.version, date: target.date, description: pattern,
    amount_cents: target.amount_cents, is_transfer: false, assignment: "auto",
    spending_group_id: null, allocations: [{ category_id: null, amount_cents: target.amount_cents }],
  }, "PUT");
  if (width === 360) {
    await page.getByRole("button", { name: "More", exact: true }).click();
    await page.getByRole("navigation", { name: "More pages" }).getByRole("button", { name: "Categories", exact: true }).click();
  } else await page.getByRole("navigation", { name: "Main navigation", exact: true }).getByRole("button", { name: "Categories", exact: true }).click();
  await page.getByRole("tab", { name: "Rules", exact: true }).click();
  await page.getByRole("button", { name: "Add rule", exact: true }).click();
  const editor = page.getByRole("dialog", { name: "Add rule", exact: true });
  const name = "Draft merchant preview " + width;
  await editor.getByLabel("Assign a merchant", { exact: true }).check();
  await editor.getByLabel("New merchant name", { exact: true }).fill(name);
  await editor.getByLabel("Description contains", { exact: true }).fill(pattern);
  await choose(page, editor, "Category", "Groceries");
  await editor.getByText("Check an example", { exact: true }).click();
  await editor.getByLabel("Example description", { exact: true }).fill(pattern + " purchase");
  await editor.getByRole("button", { name: "Check rule", exact: true }).click();
  await expect(editor.getByRole("status")).toContainText(name);
  expect((await (await page.request.get("/api/labels?kind=merchant&page=0&q=" + encodeURIComponent(name))).json()).total).toBe(0);
  await editor.getByRole("button", { name: "Save rule", exact: true }).click();
  const offer = page.getByRole("dialog", { name: "Apply this rule to previous transactions?", exact: true });
  await expect(offer).toBeVisible();
  expect((await (await page.request.get("/api/labels?kind=merchant&page=0&q=" + encodeURIComponent(name))).json()).total).toBe(1);
  await offer.getByLabel("Previous transactions", { exact: true }).selectOption("dates");
  await offer.getByLabel("From date", { exact: true }).fill("2026-10-25");
  await offer.getByLabel("To date", { exact: true }).fill("2026-10-25");
  await offer.getByRole("button", { name: "Preview changes", exact: true }).click();
  await expect(offer.getByRole("button", { name: "Apply 1 changes", exact: true })).toBeVisible();
  await offer.getByLabel("From date", { exact: true }).fill("2026-10-26");
  await expect(offer.getByRole("button", { name: "Apply 1 changes", exact: true })).toBeHidden();
  await expect(offer.getByLabel("To date", { exact: true })).toHaveAttribute("aria-invalid", "true");
  await offer.getByLabel("To date", { exact: true }).fill("2026-10-26");
  await offer.getByRole("button", { name: "Preview changes", exact: true }).click();
  await expect(offer.getByRole("heading", { name: "No changes to apply", exact: true })).toBeVisible();
  await offer.getByLabel("From date", { exact: true }).fill("2026-10-25");
  await offer.getByLabel("To date", { exact: true }).fill("2026-10-25");
  await offer.getByRole("button", { name: "Preview changes", exact: true }).click();
  await offer.getByRole("button", { name: "Apply 1 changes", exact: true }).click();
  await expect(offer).toBeHidden();
  const row = await transaction(page);
  expect(row.merchant_name).toBe(name);
  expect(row.review_state).toBe("approved");
});

test("filtered review completion preserves another account's pending queue", async ({ page }) => {
  await login(page, 1440);
  const row = await reset(page);
  const other = (await (await page.request.get("/api/transactions?account=2&acceptance=needs_category")).json()).total;
  expect(other).toBeGreaterThan(0);
  await page.getByRole("navigation", { name: "Main navigation", exact: true }).getByRole("button", { name: "Transactions", exact: true }).click();
  await page.getByRole("tab", { name: /Needs review/ }).click();
  await page.getByRole("button", { name: /^Filters(?: \(\d+\))?$/ }).click();
  await page.getByLabel("Accounts", { exact: true }).selectOption("1");
  await expect(page.getByText("Loading transactions", { exact: true })).toBeHidden();
  await page.locator(".transaction-detail").filter({ hasText: row.description }).click();
  const editor = page.getByRole("dialog", { name: "Transaction #5", exact: true });
  await choose(page, editor, "Category", "Groceries");
  await editor.getByRole("button", { name: "Save and next", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "No matches remain", exact: true })).toBeVisible();
  expect((await (await page.request.get("/api/transactions?account=2&acceptance=needs_category")).json()).total).toBe(other);
});
