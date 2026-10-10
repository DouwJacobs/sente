import { test, expect } from "@playwright/test";

async function signIn(page: import("@playwright/test").Page) {
  await page.goto("/");
  await page.getByLabel("Username", { exact: true }).fill("demo");
  await page.getByLabel("Password", { exact: true }).fill("synthetic-browser-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Spending flow", exact: true })).toBeVisible();
}

for (const theme of ["light", "dark"]) for (const width of [360, 390, 800, 1280]) {
  test(`flow labels, proportions and no scrolling ${theme} ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    await page.addInitScript(theme => localStorage.setItem("finance-theme", theme), theme);
    await page.route("**/api/dashboard?**", async route => {
      const response = await route.fetch(), body = await response.json();
      const privateScope = new URL(route.request().url()).searchParams.get("account") === "3";
      const entries = privateScope
        ? [{ group_id: 0, group_name: "No spending group", category_id: 0, category_name: "Uncategorised", spent_cents: 5000 }]
        : [
            { group_id: 1, group_name: "Day-to-day", category_id: 1, category_name: "Groceries with a very long readable category name", spent_cents: 600000 },
            { group_id: 1, group_name: "Day-to-day", category_id: 2, category_name: "Transport", spent_cents: 300000 },
            { group_id: 2, group_name: "Debt", category_id: 3, category_name: "Home loan", spent_cents: 2000000 },
          ];
      await route.fulfill({ json: { ...body, spending_breakdown: entries, spent_cents: privateScope ? 5000 : 2900000 } });
    });
    await signIn(page);
    const chart = page.locator(".spending-sankey");
    const debt = chart.locator('rect[data-group="Debt"]');
    const day = chart.locator('rect[data-group="Day-to-day"]');
    expect(Number(await debt.getAttribute("height")) / Number(await day.getAttribute("height"))).toBeCloseTo(20000 / 9000, 6);
    const geometry = await chart.evaluate(section => {
      const svg = section.querySelector("svg")!;
      const nodes = [...svg.querySelectorAll("rect[data-group]")];
      const labels = [...svg.querySelectorAll("foreignObject")];
      return nodes.map(node => {
        const name = node.getAttribute("data-group");
        const label = labels.find(el => el.querySelector("strong")?.textContent === name)!;
        const n = node.getBoundingClientRect(), l = label.getBoundingClientRect();
        return { centre: Math.abs(n.y + n.height / 2 - l.y - l.height / 2), adjacent: l.x - n.x - n.width };
      });
    });
    for (const node of geometry) {
      expect(node.centre).toBeLessThan(1);
      expect(node.adjacent).toBeGreaterThanOrEqual(7);
      expect(node.adjacent).toBeLessThan(10);
    }
    expect(await chart.evaluate(el => el.scrollWidth <= el.clientWidth + 1)).toBe(true);
    expect(await chart.locator(".sankey-chart").evaluate(el => el.scrollWidth <= el.clientWidth + 1 && el.scrollHeight <= el.clientHeight + 1)).toBe(true);
    await chart.locator("summary").click();
    await expect(chart.getByRole("table")).toContainText("Groceries with a very long readable category name");
    await expect(chart.getByRole("table")).toContainText("20");
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.getByLabel("Accounts", { exact: true }).selectOption("3");
    await expect(chart.locator('rect[data-group="Debt"]')).toHaveCount(0);
    await expect(chart.locator(".sankey-total")).toContainText("50,00");
    await chart.locator("summary").click();
    await expect(chart.getByRole("table")).toContainText("Uncategorised");
  });
}

test("real authorized breakdown reconciles with dashboard across account and period changes", async ({ page }) => {
  await signIn(page);
  const verify = async () => {
    const scope = await page.getByLabel("Accounts", { exact: true }).inputValue();
    const period = await page.getByLabel("Budget period", { exact: true }).inputValue();
    const response = await page.request.get("/api/dashboard?period=" + period + "&account=" + scope + "&group_page=1&category_page=1");
    expect(response.ok()).toBe(true);
    const data = await response.json();
    expect(data.spending_breakdown.reduce((sum: number, row: { spent_cents: number }) => sum + row.spent_cents, 0)).toBe(data.spent_cents);
    await expect(page.locator(".sankey-total")).toContainText(new Intl.NumberFormat("en-ZA", { style: "currency", currency: "ZAR" }).format(data.spent_cents / 100));
  };
  await verify();
  await page.getByLabel("Accounts", { exact: true }).selectOption("3");
  await verify();
  const periods = await page.getByLabel("Budget period", { exact: true }).locator("option").evaluateAll(options => options.map(option => (option as HTMLOptionElement).value));
  if (periods.length > 1) {
    await page.getByLabel("Budget period", { exact: true }).selectOption(periods[1]);
    await verify();
  }
});

test("net refunds, zero flows, empty, loading and request failures", async ({ page }) => {
  let mode = "refund";
  let release: (() => void) | undefined;
  await page.route("**/api/dashboard?**", async route => {
    const response = await route.fetch(), body = await response.json();
    if (mode === "failure") return route.fulfill({ status: 500, json: { error: "Synthetic dashboard failure" } });
    if (mode === "loading") await new Promise<void>(resolve => { release = resolve; });
    const entries = mode === "empty" ? [] : mode === "zero"
      ? [{ group_id: 1, group_name: "Debt", category_id: 1, category_name: "Loan", spent_cents: 0 }]
      : [
          { group_id: 1, group_name: "Debt", category_id: 1, category_name: "Loan", spent_cents: 100000 },
          { group_id: 2, group_name: "Day-to-day", category_id: 2, category_name: "Refunded category", spent_cents: -40000 },
        ];
    await route.fulfill({ json: { ...body, spending_breakdown: entries, spent_cents: mode === "zero" || mode === "empty" ? 0 : 60000 } });
  });
  await signIn(page);
  const chart = page.locator(".spending-sankey");
  await expect(chart).toContainText("Net refunds");
  await chart.locator("summary").click();
  await expect(chart.getByRole("table")).toContainText("-66,7%");
  await expect(chart.locator("svg rect[data-cents]")).toHaveCount(2);
  mode = "zero";
  await page.getByLabel("Accounts", { exact: true }).selectOption("3");
  await expect(chart).toContainText("No positive spending flows");
  await expect(chart.locator(".sankey-chart > svg")).toHaveCount(0);
  mode = "empty";
  await page.getByLabel("Accounts", { exact: true }).selectOption("");
  await expect(chart.getByText("No spending yet", { exact: true })).toBeVisible();
  mode = "loading";
  await page.getByLabel("Accounts", { exact: true }).selectOption("3");
  await expect(page.getByText("Loading this period", { exact: true })).toBeVisible();
  await expect(chart).toHaveCount(0);
  await expect.poll(() => Boolean(release)).toBe(true);
  mode = "refund"; release!();
  await expect(chart).toContainText("Net refunds");
  mode = "failure";
  await page.getByLabel("Accounts", { exact: true }).selectOption("");
  await expect(page.getByText("Dashboard unavailable", { exact: true })).toBeVisible();
  await expect(chart).toHaveCount(0);
  mode = "refund";
  await page.getByRole("button", { name: "Retry dashboard", exact: true }).click();
  await expect(chart).toContainText("Net refunds");
});
