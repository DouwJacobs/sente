import { desktopPhoneCases } from './coverage-cases';
import { test, expect } from "@playwright/test";

for (const {width,theme} of desktopPhoneCases) {
    test(`About and safe issue reporting at ${width}px in ${theme}`, async ({ page }) => {
      await page.setViewportSize({ width, height: 900 });
      await page.addInitScript((value) => localStorage.setItem("finance-theme", value), theme);
      await page.goto("/");
      await page.getByLabel("Username", { exact: true }).fill("demo");
      await page.getByLabel("Password", { exact: true }).fill("synthetic-browser-password");
      await page.getByRole("button", { name: "Sign in", exact: true }).click();
      await expect(page.getByRole("heading", { name: "Dashboard", exact: true })).toBeVisible();
      if (width === 1440) {
        const version = page.getByRole("button", { name: "About Sente", exact: true });
        await expect(version).toHaveText("Development");
        const compact = await version.boundingBox();
        expect(compact!.height).toBeLessThanOrEqual(44);
        await version.click();
      } else {
        await page.getByRole("navigation", { name: "Mobile navigation" }).getByRole("button", { name: "More", exact: true }).click();
        await page.getByRole("navigation", { name: "More pages" }).getByRole("button", { name: "Settings", exact: true }).click();
        await page.getByRole("tab", { name: "General", exact: true }).focus();
        await page.keyboard.press("End");
      }
      await expect(page.getByRole("tab", { name: "About", exact: true })).toHaveAttribute("aria-selected", "true");
      await expect(page.getByRole("heading", { name: "About Sente", exact: true })).toBeVisible();
      await expect(page.locator(".about-build")).toBeVisible();
      await expect(page.getByRole("link", { name: "GNU GPL v3", exact: true })).toHaveAttribute("href", /\/LICENSE$/);
      const resources = await page.locator(".about-resources").boundingBox();
      const installation = await page.locator(".about-installation").boundingBox();
      if (width === 1440) {
        expect(Math.abs(resources!.y - installation!.y)).toBeLessThan(2);
      } else {
        expect(installation!.y).toBeGreaterThanOrEqual(resources!.y + resources!.height);
      }
      expect(await page.locator("html").getAttribute("data-theme")).toBe(theme);
      const report = page.getByRole("link", { name: "Report an issue", exact: true });
      const url = new URL((await report.getAttribute("href"))!);
      expect(url.origin + url.pathname).toBe("https://github.com/DouwJacobs/sente/issues/new");
      expect(url.searchParams.get("body")).toContain("Version:");
      expect(url.searchParams.get("body")).not.toContain("Everyday account");
      expect(url.searchParams.get("body")).not.toContain("synthetic-browser-password");
      await expect(report).toHaveAttribute("rel", "noopener noreferrer");
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
      // No report is sent or external tab opened during these checks.
    });
}

test("About retries a failed metadata request without losing the report link", async ({ page }) => {
  await page.route("**/api/build", (route) => route.fulfill({ status: 503, json: { error: "Unavailable" } }));
  await page.goto("/");
  await page.getByLabel("Username", { exact: true }).fill("demo");
  await page.getByLabel("Password", { exact: true }).fill("synthetic-browser-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await page.getByRole("button", { name: "About Sente", exact: true }).click();
  await expect(page.getByRole("tabpanel", { name: "About", exact: true }).getByText("Build information could not be loaded.")).toBeVisible();
  expect(new URL((await page.getByRole("link", { name: "Report an issue", exact: true }).getAttribute("href"))!).searchParams.get("body")).toContain("unavailable");
  await page.unroute("**/api/build");
  await page.getByRole("button", { name: "Retry", exact: true }).click();
  await expect(page.locator(".about-build")).toBeVisible();
});

for (const { width, theme } of [{width:360,theme:"light"},{width:360,theme:"dark"},{width:1440,theme:"light"},{width:1440,theme:"dark"}]) {
  test(`Application releases, retry and backup guidance at ${width}px in ${theme}`, async ({ page }) => {
    let state = "available";
    let attempts = 0;
    await page.setViewportSize({ width, height: 900 });
    await page.addInitScript(value => localStorage.setItem("finance-theme", value), theme);
    await page.route("**/api/build/update", route => {
      attempts++;
      return route.fulfill({ json: {
        state, channel: "beta", available_version: state === "current" ? undefined : "1.0.0-beta.10",
        release_url: state === "current" ? undefined : "https://github.com/DouwJacobs/sente/releases/tag/v1.0.0-beta.10",
        checked_at: "2026-10-09T00:00:00Z", stale: state === "unavailable",
        message: state === "unavailable" ? "Release check unavailable. Try again shortly." : undefined,
      } });
    });
    await page.goto("/");
    await page.getByLabel("Username", { exact: true }).fill("demo");
    await page.getByLabel("Password", { exact: true }).fill("synthetic-browser-password");
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Dashboard", exact: true })).toBeVisible();
    if (width === 1440) {
      await page.getByRole("button", { name: "About Sente, update available: 1.0.0-beta.10", exact: true }).click();
    } else {
      await page.getByRole("navigation", { name: "Mobile navigation" }).getByRole("button", { name: "More", exact: true }).click();
      const settings = page.getByRole("navigation", { name: "More pages" }).getByRole("button", { name: "Settings Update available", exact: true });
      await expect(settings).toBeVisible();
      await settings.click();
      await page.getByRole("tab", { name: "About", exact: true }).click();
    }
    const updates = page.getByRole("region", { name: "Application updates", exact: true });
    await expect(updates.getByRole("status")).toContainText("Update available: 1.0.0-beta.10");
    await expect(updates.getByRole("link", { name: "Release notes: 1.0.0-beta.10" })).toHaveAttribute("href", /releases\/tag\/v1.0.0-beta.10$/);
    await expect(updates.getByRole("link", { name: "Upgrade and backup guide" })).toHaveAttribute("href", /#upgrade-with-a-pre-upgrade-backup$/);
    await expect(updates).toContainText("take a backup with the installed image");
    const check = updates.getByRole("button", { name: "Check for updates", exact: true });
    const checkBounds = await check.boundingBox();
    expect(checkBounds!.height).toBeGreaterThanOrEqual(44);
    await check.focus();
    state = "unavailable";
    await page.keyboard.press("Enter");
    await expect(updates.getByRole("status")).toContainText("Release check unavailable");
    await expect(updates).toContainText("Last successful check (outdated)");
    await expect(updates.getByRole("link", { name: "Previously found release: 1.0.0-beta.10" })).toBeVisible();
    expect(await page.getByRole("alert").count()).toBe(0);
    state = "current";
    await check.click();
    await expect(updates.getByRole("status")).toContainText("Up to date for this release channel");
    await expect(updates.getByRole("link", { name: /Release notes|Previously found release/ })).toHaveCount(0);
    state = "unknown";
    await check.click();
    await expect(updates.getByRole("status")).toContainText("Update status unknown");
    state = "unsupported";
    await check.click();
    await expect(updates.getByRole("status")).toContainText("Release checks unsupported for this build");
    await expect(check).toHaveCount(0);
    expect(attempts).toBe(5);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
  });
}
