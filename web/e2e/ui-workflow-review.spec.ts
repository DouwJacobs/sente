import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/");
  await page.getByLabel("Username", { exact: true }).fill("demo");
  await page.getByLabel("Password", { exact: true }).fill("synthetic-browser-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Dashboard", exact: true })).toBeVisible();
}
async function navigate(page: Page, name: string) {
  await page.getByRole("navigation", { name: "Main navigation", exact: true }).getByRole("button", { name, exact: true }).click();
}
async function openTransaction(page: Page) {
  await navigate(page, "Transactions");
  await expect(page.getByText("Loading transactions", { exact: true })).toBeHidden();
  await page.locator(".transaction-detail").first().click();
  return page.getByRole("dialog").filter({ has: page.getByRole("heading", { name: /^Transaction #/ }) });
}

test("settings read isolation, pending lock, dirty draft and retry", async ({ page }) => {
  let release!: () => void;
  const gate = new Promise<void>(resolve => { release = resolve; });
  let first = true;
  let saved = 7;
  const writes: number[] = [];
  await page.route("**/api/settings", async route => {
    if (route.request().method() === "PUT") {
      saved = route.request().postDataJSON().start_day;
      writes.push(saved);
      return route.fulfill({ json: {} });
    }
    if (first) { first = false; await gate; }
    return route.fulfill({ json: { start_day: saved } });
  });
  let backupsFail = true;
  await page.route("**/api/backups", route => route.fulfill(backupsFail ? { status: 503, json: { error: "Synthetic backup outage" } } : { json: { items: [] } }));
  await login(page);
  await navigate(page, "Settings");
  const day = page.getByLabel("Default period start day", { exact: true });
  await expect(day).toBeDisabled();
  await expect(page.getByRole("button", { name: "Save default" })).toBeDisabled();
  expect(writes).toEqual([]);
  release();
  await expect(day).toHaveValue("7");
  await day.fill("9");
  await page.getByRole("tab", { name: "Backups", exact: true }).click();
  await expect(page.getByText("No successful backup recorded yet.")).toBeHidden();
  backupsFail = false;
  await page.getByRole("button", { name: "Retry backups" }).click();
  await expect(page.getByText("No successful backup recorded yet.")).toBeVisible();
  await page.getByRole("button", { name: "Back up now", exact: true }).click();
  await expect(page.getByRole("button", { name: "Back up now", exact: true })).toBeEnabled();
  await page.getByRole("tab", { name: "General", exact: true }).click();
  await expect(day).toHaveValue("9");
  await page.getByRole("button", { name: "Save default" }).click();
  await expect.poll(() => writes).toEqual([9]);
  await expect(day).toBeEnabled();
  await expect(day).toHaveValue("9");
});

test("budget draft ignores a delayed preview from previous dates", async ({ page }) => {
  let release!: () => void;
  const gate = new Promise<void>(resolve => { release = resolve; });
  await page.route("**/api/periods/new/preview?*", async route => {
    await gate;
    return route.fulfill({ json: { total: 123, manual_outside: 0, affected: [], preview_token: "old-draft" } });
  });
  await login(page);
  await navigate(page, "Budgets");
  await page.getByRole("button", { name: "New period", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "New budget period", exact: true });
  await dialog.getByLabel("Period name", { exact: true }).fill("Synthetic future period");
  await dialog.getByLabel("Start date", { exact: true }).fill("2028-01-01");
  await dialog.getByLabel("End date (inclusive)", { exact: true }).fill("2028-01-31");
  const request = page.waitForRequest("**/api/periods/new/preview?*");
  await dialog.getByRole("button", { name: "Preview affected transactions" }).click();
  await request;
  await dialog.getByLabel("End date (inclusive)", { exact: true }).fill("2028-02-01");
  const response = page.waitForResponse("**/api/periods/new/preview?*");
  release();
  await response;
  await expect(dialog.getByRole("button", { name: "Preview affected transactions" })).toBeEnabled();
  await expect(dialog.getByText(/123 transactions/)).toBeHidden();
  await expect(dialog.getByRole("button", { name: "Save period", exact: true })).toBeDisabled();
});

for (const failure of ["server", "network"]) test("session startup retries " + failure + " failure", async ({ page }) => {
  await login(page);
  await page.route("**/api/me", route => failure === "network" ? route.abort() : route.fulfill({ status: 503, json: { error: "Synthetic session outage" } }));
  await page.reload();
  await expect(page.getByRole("button", { name: /Retry/ })).toBeVisible();
  await expect(page.getByRole("button", { name: "Sign in", exact: true })).toBeHidden();
  await page.unroute("**/api/me");
  await page.getByRole("button", { name: /Retry/ }).click();
  await expect(page.getByRole("heading", { name: "Dashboard", exact: true })).toBeVisible();
});

test("51 account options have two pages and support off-page selection", async ({ page }) => {
  await login(page);
  await page.route("**/api/accounts/manage?*", route => {
    const params = new URL(route.request().url()).searchParams;
    const all = Array.from({ length: 51 }, (_, index) => ({ id: 9000 + index, name: "Synthetic account " + index, household: 0 }));
    const id = Number(params.get("id"));
    const items = id ? all.filter(a => a.id === id) : all.slice(Number(params.get("page")) * 50, Number(params.get("page")) * 50 + 50);
    return route.fulfill({ json: { items, total: 51, list_version: "options" } });
  });
  await navigate(page, "Settings");
  await page.getByRole("tab", { name: "Users & access", exact: true }).click();
  const panel = page.locator("#settings-panel-access");
  await panel.getByText("Find more account options", { exact: true }).click();
  await expect(panel.getByText("Page 1 of 2 · 51 items", { exact: true })).toBeVisible();
  const details = panel.locator("details").filter({ has: page.getByText("Find more account options", { exact: true }) });
  await details.getByRole("button", { name: "Next", exact: true }).click();
  await expect(details.getByText("Page 2 of 2 · 51 items", { exact: true })).toBeVisible();
  await expect(details.getByRole("button", { name: "Next", exact: true })).toBeDisabled();
  await panel.getByLabel("Account", { exact: true }).selectOption("9050");
  await expect(panel.getByLabel("Account", { exact: true })).toHaveValue("9050");
});

test("category search suppresses creation and stale results while pending or failed", async ({ page }) => {
  await login(page);
  const editor = await openTransaction(page);
  await editor.getByLabel("Category", { exact: true }).first().click();
  const picker = page.getByRole("dialog", { name: "Select category", exact: true });
  let release!: () => void;
  const gate = new Promise<void>(resolve => { release = resolve; });
  let fail = true;
  let creates = 0;
  page.on("request", request => { if (request.method() === "POST" && new URL(request.url()).pathname === "/api/categories") creates++; });
  await page.route("**/api/categories?*", async route => {
    const query = new URL(route.request().url()).searchParams.get("q");
    if (query === "Remote existing") {
      await gate;
      return route.fulfill(fail ? { status: 503, json: { error: "Synthetic category outage" } } : { json: { items: [{ id: 7777, name: "Remote existing", kind: "expense" }], total: 1 } });
    }
    return route.continue();
  });
  await picker.getByLabel("Search category", { exact: true }).fill("Remote existing");
  await picker.getByLabel("Search category", { exact: true }).press("Enter");
  expect(creates).toBe(0);
  await expect(picker.getByRole("heading", { name: "No matches" })).toBeHidden();
  release();
  await expect(picker.getByRole("button", { name: "Retry", exact: true })).toBeVisible();
  await expect(picker.getByRole("button", { name: /Create “/ })).toBeHidden();
  fail = false;
  await picker.getByRole("button", { name: "Retry", exact: true }).click();
  await picker.getByRole("button", { name: "Remote existing Expense", exact: true }).click();
  await expect(editor.getByLabel("Category", { exact: true }).first()).toContainText("Remote existing");
  expect(creates).toBe(0);
});

test("inbox preserves page on focus and clamps after messages disappear", async ({ page }) => {
  await login(page);
  let total = 41;
  const pages: number[] = [];
  await page.route("**/api/notifications?*", route => {
    const current = Number(new URL(route.request().url()).searchParams.get("page"));
    pages.push(current);
    return route.fulfill({ json: { total, unread_count: total, items: Array.from({ length: Math.max(0, Math.min(20, total - current * 20)) }, (_, i) => ({ id: current * 20 + i + 1, title: "Synthetic message " + (current * 20 + i + 1), message: "Synthetic content", severity: "info", type: "system", created_at: 1794477600, occurred_at: 1794477600, source_kind: "system", source_id: 0, read_at: null, dismissible: 0 })) } });
  });
  await page.getByRole("button", { name: /^Notifications,/ }).click();
  await page.getByRole("button", { name: "Next notifications" }).click();
  await expect(page.getByText("Page 2 of 3", { exact: true })).toBeVisible();
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  await expect.poll(() => pages.filter(p => p === 1).length).toBeGreaterThan(1);
  await expect(page.getByText("Page 2 of 3", { exact: true })).toBeVisible();
  total = 1;
  await page.evaluate(() => document.dispatchEvent(new Event("visibilitychange")));
  await expect.poll(() => pages.at(-1)).toBe(0);
  await expect(page.getByRole("article", { name: "Synthetic message 1", exact: true })).toBeVisible();
  await expect(page.getByRole("article", { name: "Synthetic message 21", exact: true })).toBeHidden();
});

test("MCP initial read can retry without leaving the tab", async ({ page }) => {
  await login(page);
  await page.route("**/api/mcp/settings", route => route.fulfill({ status: 503, json: { error: "Synthetic MCP outage" } }));
  await navigate(page, "Settings");
  await page.getByRole("tab", { name: "MCP", exact: true }).click();
  await expect(page.getByText("Loading MCP settings", { exact: true })).toBeHidden();
  await expect(page.getByRole("button", { name: "Retry MCP settings" })).toBeVisible();
  await page.unroute("**/api/mcp/settings");
  await page.getByRole("button", { name: "Retry MCP settings" }).click();
  await expect(page.getByLabel("MCP server endpoint", { exact: true })).toBeVisible();
});

test("category dependency retry preserves rename draft", async ({ page }) => {
  await login(page);
  await page.route("**/api/categories/*/dependencies", route => route.fulfill({ status: 503, json: { error: "Synthetic dependency outage" } }));
  await navigate(page, "Categories");
  await page.getByRole("button", { name: "Edit Groceries", exact: true }).click();
  const editor = page.getByRole("dialog", { name: "Edit category · Groceries", exact: true });
  await editor.getByLabel("Category name", { exact: true }).fill("Retained grocery name");
  await expect(editor.getByLabel("Archived", { exact: true })).toBeDisabled();
  await expect(editor.getByRole("button", { name: "Save category" })).toBeDisabled();
  await page.unroute("**/api/categories/*/dependencies");
  await editor.getByRole("button", { name: "Retry dependencies" }).click();
  await expect(editor.getByRole("button", { name: "Save category" })).toBeEnabled();
  await expect(editor.getByLabel("Category name", { exact: true })).toHaveValue("Retained grocery name");
});

test("unknown import counts do not erase a verified review count", async ({ page }) => {
  await login(page);
  await page.route("**/api/imports?page=0", route => route.fulfill({ status: 503, json: { error: "Synthetic count outage" } }));
  await page.reload();
  await expect(page.getByRole("heading", { name: "Dashboard", exact: true })).toBeVisible();
  await navigate(page, "Transactions");
  await expect(page.getByRole("tab", { name: "Import activity", exact: true })).toBeVisible();
  await expect(page.getByRole("tab", { name: /Needs review \([1-9]/ })).toBeVisible();
});

test("unknown bank connection offers in-place recovery", async ({ page }) => {
  await login(page);
  await page.route("**/api/fnb?summary=1", route => route.fulfill({ status: 503, json: { error: "Synthetic connection outage" } }));
  await navigate(page, "Transactions");
  await page.getByRole("tab", { name: /Import activity/ }).click();
  await expect(page.getByRole("button", { name: "Connect FNB", exact: true })).toBeHidden();
  await expect(page.getByRole("button", { name: "Connection settings", exact: true })).toBeVisible();
  await page.unroute("**/api/fnb?summary=1");
  await page.getByRole("button", { name: "Retry connection status" }).click();
  await expect(page.getByRole("button", { name: "Retry connection status" })).toBeHidden();
});


test("bulk import stops on failure, retains remaining files and avoids duplicate commits", async ({ page }) => {
  await login(page);
  const committed: number[] = [];
  const attempts: number[] = [];
  let failSecond = true;
  let finishRetry!: () => void;
  const retryGate = new Promise<void>(resolve => { finishRetry = resolve; });
  const report = (id: number) => ({
    id, name: "Synthetic " + id, format: "fnb-live", bank_id: "12345678901",
    account_id: 1, account_name: "Synthetic file " + id, currency: "ZAR", total: 1,
    candidate_count: 1, error_count: 0, classification_version: "rules",
    rows: [{ row: 1, date: "2026-10-01", description: "Synthetic purchase",
      amount_cents: -100, category_id: 1, duplicate: "possible",
      candidates: [{ id: 99, date: "2026-10-01", description: "Synthetic purchase", amount_cents: -100 }] }],
  });
  await page.route("**/api/fnb?summary=1", route => route.fulfill({ json: { connection: { state: "ready" } } }));
  await page.route("**/api/fnb/transactions?*", route => route.fulfill({ json: [9101, 9102, 9103].map(report) }));
  await page.route("**/api/imports**", async route => {
    const id = Number(new URL(route.request().url()).pathname.split("/")[3]);
    if (route.request().method() === "POST") {
      attempts.push(id);
      if (id === 9102 && failSecond) return route.fulfill({ status: 503, json: { error: "Synthetic second-file failure" } });
      if (id === 9102) await retryGate;
      committed.push(id);
      return route.fulfill({ json: { inserted: 1, skipped: 0 } });
    }
    if (id) return route.fulfill({ json: report(id) });
    return route.fulfill({ json: { items: [], total: 0, pending_total: 0 } });
  });
  await navigate(page, "Transactions");
  await page.getByRole("tab", { name: /Import activity/ }).click();
  await page.getByRole("button", { name: "Get transactions", exact: true }).click();
  await expect(page.locator("section.preview")).toHaveCount(3);
  for (const preview of await page.locator("section.preview").all()) {
    const summary = preview.locator("summary").first();
    if (!(await preview.locator("details").first().getAttribute("open") !== null)) await summary.click();
    await preview.getByLabel("Decision for row 1", { exact: true }).selectOption("keep");
  }
  await page.getByRole("button", { name: "Send 3 accounts to review", exact: true }).click();
  await expect.poll(() => attempts).toEqual([9101, 9102]);
  await expect(page.locator("section.preview")).toHaveCount(2);
  await expect(page.getByRole("tab", { name: /Import activity/ })).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("button", { name: "View successfully added transactions" })).toBeVisible();
  failSecond = false;
  await page.getByRole("button", { name: "Send 2 accounts to review", exact: true }).click();
  await expect(page.getByRole("button", { name: "View successfully added transactions" })).toBeDisabled();
  finishRetry();
  await expect.poll(() => attempts).toEqual([9101, 9102, 9102, 9103]);
  expect(committed).toEqual([9101, 9102, 9103]);
  await expect(page.getByRole("tab", { name: /Import activity/ })).toHaveAttribute("aria-selected", "false");
});

test("manual assignment hydrates an old period and can search another page", async ({ page }) => {
  await login(page);
  await page.route("**/api/transactions?id=*", async route => {
    const response = await route.fetch();
    const body = await response.json();
    return route.fulfill({ response, json: { ...body, items: body.items.map((row: any) => ({ ...row, assignment: "manual", period_id: 8100 })) } });
  });
  await page.route("**/api/periods?*", route => {
    const params = new URL(route.request().url()).searchParams;
    const items = Array.from({ length: 101 }, (_, i) => ({ id: 8000 + i, name: "Archive period " + i, start_date: "2020-01-01", end_date: "2020-01-31" }));
    const id = Number(params.get("id"));
    const query = params.get("q") || "";
    const matches = items.filter(p => (!id || p.id === id) && p.name.includes(query));
    const size = Number(params.get("page_size") || 20);
    const offset = Number(params.get("page") || 0) * size;
    return route.fulfill({ json: { items: matches.slice(offset, offset + size), total: matches.length } });
  });
  const editor = await openTransaction(page);
  const assignmentSummary = editor.locator("summary").filter({ hasText: /^Budget assignment/ });
  if (await assignmentSummary.locator("xpath=..").getAttribute("open") === null) await assignmentSummary.click();
  await expect(editor.getByLabel("Assigned period", { exact: true })).toHaveValue("8100");
  await expect(editor.getByLabel("Assigned period", { exact: true }).getByRole("option", { name: /Archive period 100/ })).toHaveCount(1);
  await editor.getByText("Find more assigned period options", { exact: true }).click();
  await editor.getByLabel("Search assigned period", { exact: true }).fill("Archive period 99");
  await expect(editor.getByLabel("Assigned period", { exact: true }).getByRole("option", { name: /Archive period 99/ })).toHaveCount(1);
  await editor.getByLabel("Assigned period", { exact: true }).selectOption("8099");
  await expect(editor.getByLabel("Assigned period", { exact: true })).toHaveValue("8099");
});

test("editable account lookup permits upload beyond initial viewer-only slice", async ({ page }) => {
  await login(page);
  await page.route("**/api/accounts?*", async route => {
    const url = new URL(route.request().url());
    if (url.searchParams.get("role") === "editor") return route.fulfill({ json: { items: [{ id: 7100, name: "Late editable account", role: "editor", bank_id: "77777777777" }], total: 1 } });
    const response = await route.fetch();
    const body = await response.json();
    return route.fulfill({ response, json: { ...body, items: body.items.map((a: any) => ({ ...a, role: "viewer" })) } });
  });
  await page.reload();
  await expect(page.getByRole("heading", { name: "Dashboard", exact: true })).toBeVisible();
  await navigate(page, "Transactions");
  await page.getByRole("tab", { name: /Import activity/ }).click();
  await page.getByText("Upload a bank export", { exact: true }).click();
  await expect(page.getByLabel("Import into account", { exact: true })).toBeVisible();
  await page.getByLabel("Import into account", { exact: true }).selectOption("7100");
  await expect(page.getByLabel("Import into account", { exact: true })).toHaveValue("7100");
});


test("closing a pending budget preview does not populate a replacement editor", async ({ page }) => {
  await login(page);
  let release!: () => void;
  const gate = new Promise<void>(resolve => { release = resolve; });
  await page.route("**/api/periods/new/preview?*", async route => {
    await gate;
    return route.fulfill({ json: { total: 999, manual_outside: 0, affected: [], preview_token: "closed-draft" } });
  });
  await navigate(page, "Budgets");
  await page.getByRole("button", { name: "New period", exact: true }).click();
  const editor = page.getByRole("dialog", { name: "New budget period", exact: true });
  await editor.getByLabel("Period name", { exact: true }).fill("Old draft");
  const request = page.waitForRequest("**/api/periods/new/preview?*");
  await editor.getByRole("button", { name: "Preview affected transactions" }).click();
  await request;
  await editor.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(editor).toBeHidden();
  await page.getByRole("button", { name: "New period", exact: true }).click();
  await editor.getByLabel("Period name", { exact: true }).fill("Replacement draft");
  const response = page.waitForResponse("**/api/periods/new/preview?*");
  release();
  await response;
  await expect(editor.getByRole("button", { name: "Preview affected transactions" })).toBeEnabled();
  await expect(editor.getByLabel("Period name", { exact: true })).toHaveValue("Replacement draft");
  await expect(editor.getByText(/999 transactions/)).toBeHidden();
  await expect(editor.getByRole("button", { name: "Save period", exact: true })).toBeDisabled();
});

test("successful MCP decision followed by read failure retries only the read", async ({ page }) => {
  await login(page);
  let decided = false;
  let failRead = false;
  let mutations = 0;
  await page.route("**/api/mcp/settings", route => route.fulfill(failRead ? {
    status: 503, json: { error: "Synthetic post-decision outage" },
  } : { json: { endpoint: "https://sente.example/api/mcp", accounts: [], connections: [],
    proposals: [{ id: "synthetic-proposal", status: decided ? "approved" : "pending", operation: "edit_transactions",
      token_name: "Synthetic agent", expires_at: 1800000000, payload: { change: {}, transactions: [] } }] } }));
  await page.route("**/api/mcp/proposals/synthetic-proposal", route => {
    mutations++;
    decided = true;
    failRead = true;
    return route.fulfill({ json: {} });
  });
  await navigate(page, "Settings");
  await page.getByRole("tab", { name: "MCP", exact: true }).click();
  await page.getByRole("button", { name: "Approve exact changes", exact: true }).click();
  await expect(page.getByRole("button", { name: "Retry MCP settings" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Approve exact changes", exact: true })).toBeHidden();
  failRead = false;
  await page.getByRole("button", { name: "Retry MCP settings" }).click();
  await expect(page.locator(".mcp-proposal")).toContainText("approved");
  expect(mutations).toBe(1);
});



