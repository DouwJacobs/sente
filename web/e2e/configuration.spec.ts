import { test, expect, type Page } from "@playwright/test";
async function login(page: Page, width: number) {
  await page.setViewportSize({ width, height: 900 });
  await page.goto("/");
  await page.getByLabel("Username", { exact: true }).fill("demo");
  await page
    .getByLabel("Password", { exact: true })
    .fill("synthetic-browser-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Dashboard", exact: true }),
  ).toBeVisible();
  if (width === 360) {
    await page
      .getByRole("navigation", { name: "Mobile navigation" })
      .getByRole("button", { name: "More", exact: true })
      .click();
    await page
      .getByRole("navigation", { name: "More pages" })
      .getByRole("button", { name: "Settings", exact: true })
      .click();
  } else
    await page
      .getByRole("navigation", { name: "Main navigation", exact: true })
      .getByRole("button", { name: "Settings", exact: true })
      .click();
  await page.getByRole("tab", { name: "Configuration", exact: true }).click();
}
for (const width of [1440, 360])
  test(`configuration file import, coexistence, replacement and export at ${width}px`, async ({
    page,
  }) => {
    const errors: string[] = [];
    page.on("pageerror", (e) => errors.push(e.message));
    await login(page, width);
    const portable = await page.getByRole("heading", { name: "Portable configuration", exact: true }).locator("..").locator("..").boundingBox();
    const forms = await page.locator(".configuration-source-forms").boundingBox();
    const sources = await page.getByRole("heading", { name: "Configuration sources", exact: true }).locator("..").boundingBox();
    expect(portable).not.toBeNull();
    expect(forms).not.toBeNull();
    expect(sources).not.toBeNull();
    const above = forms!.y - (portable!.y + portable!.height);
    const below = sources!.y - (forms!.y + forms!.height);
    expect(above).toBeGreaterThan(0);
    expect(below).toBeCloseTo(above, 0);
    const suffix = String(width);
    const group = "Synthetic configuration " + suffix;
    const merchant = "Portable merchant " + suffix;
    const config = {
      version: 2,
      spending_groups: [{ name: group, color: "teal" }],
      categories: [
        {
          name: "Portable supplies " + suffix,
          group_name: "",
          kind: "expense",
        },
      ],
      merchants: [
        {
          name: merchant,
          category: "Portable supplies " + suffix,
          spending_group: group,
        },
      ],
      merchant_rules: [
        {
          merchant,
          pattern: "PORTABLE SHOP " + suffix,
          direction: "debit",
          priority: 0,
          enabled: true,
        },
      ],
      rules: [
        {
          pattern: "PORTABLE SHOP " + suffix,
          category: "Portable supplies " + suffix,
          builtin: true,
          direction: "debit",
          priority: -10,
          enabled: true,
        },
      ],
    };
    const input = page.getByLabel("Configuration JSON file", { exact: true });
    await input.setInputFiles({
      name: `portable-${suffix}.json`,
      mimeType: "application/json",
      buffer: Buffer.from(JSON.stringify(config)),
    });
    // Drafts survive tab navigation.
    await page
      .getByLabel("Source name", { exact: true })
      .fill("Unsaved repository " + suffix);
    await page.getByRole("tab", { name: "General", exact: true }).click();
    await page.getByRole("tab", { name: "Configuration", exact: true }).click();
    await expect(page.getByLabel("Source name", { exact: true })).toHaveValue(
      "Unsaved repository " + suffix,
    );
    await page
      .getByRole("button", { name: "Validate and preview file", exact: true })
      .click();
    const preview = page.getByRole("region", {
      name: "Configuration import preview",
    });
    await expect(preview).toBeVisible();
    await expect(
      preview.getByText(
        "5 changes, including 0 replacements. Nothing has been imported yet.",
      ),
    ).toBeVisible();
    const before = await page.evaluate(async (name) => {
      const result = await fetch("/api/labels?kind=merchant").then((r) =>
        r.json(),
      );
      return JSON.stringify(result).includes(name);
    }, merchant);
    expect(before).toBe(false);
    await preview
      .getByRole("button", { name: "Import configuration", exact: true })
      .click();
    await expect(preview).toHaveCount(0);
    const source = page
      .locator(".configuration-source")
      .filter({
        has: page.getByRole("heading", {
          name: `portable-${suffix}.json`,
          exact: true,
        }),
      });
    await expect(source).toBeVisible();
    // Invalid JSON appears beneath the file field and leaves imported configuration intact.
    await input.setInputFiles({
      name: "invalid.json",
      mimeType: "application/json",
      buffer: Buffer.from('{"version":99}'),
    });
    await page
      .getByRole("button", { name: "Validate and preview file", exact: true })
      .click();
    await expect(
      page.getByText("unsupported ruleset version 99", { exact: true }),
    ).toBeVisible();
    await page
      .getByLabel("File source", { exact: true })
      .selectOption({ label: `portable-${suffix}.json` });
    config.spending_groups[0].color = "purple";
    await input.setInputFiles({
      name: `updated-${suffix}.json`,
      mimeType: "application/json",
      buffer: Buffer.from(JSON.stringify(config)),
    });
    await page
      .getByRole("button", { name: "Validate and preview file", exact: true })
      .click();
    await expect(
      preview.getByText(
        "1 changes, including 1 replacements. Nothing has been imported yet.",
      ),
    ).toBeVisible();
    await preview
      .getByRole("button", { name: "Import configuration", exact: true })
      .click();
    await expect(preview).toBeVisible();
    await preview.getByRole("checkbox").check();
    await preview
      .getByRole("button", { name: "Import configuration", exact: true })
      .click();
    await expect(preview).toHaveCount(0);
    const downloadPromise = page.waitForEvent("download");
    await page
      .getByRole("button", { name: "Export configuration", exact: true })
      .click();
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toBe("sente-configuration.json");
    const exported = await page.evaluate(async () =>
      fetch("/api/configuration/export").then((r) => r.json()),
    );
    expect(exported.version).toBe(2);
    expect(
      exported.merchants.some((m: { name: string }) => m.name === merchant),
    ).toBe(true);
    expect(exported.transactions).toBeUndefined();
    // Removing source tracking preserves its imported entities.
    await source
      .getByRole("button", { name: "Forget source", exact: true })
      .click();
    await expect(source).toHaveCount(0);
    const kept = await page.evaluate(async () =>
      fetch("/api/configuration/export").then((r) => r.json()),
    );
    expect(
      kept.merchants.some((m: { name: string }) => m.name === merchant),
    ).toBe(true);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    expect(errors).toEqual([]);
  });
