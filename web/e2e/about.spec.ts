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
