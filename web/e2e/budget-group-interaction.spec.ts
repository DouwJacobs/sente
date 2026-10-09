import { test, expect } from "@playwright/test";
import { start, colorToken } from "./responsive-helpers";

for (const touch of [false, true]) {
  test.describe(touch ? "touch spending groups" : "mouse spending groups", () => {
    test.use({ hasTouch: touch, isMobile: touch });
    for (const theme of ["light", "dark"]) {
      test(`whole card hover clears while expanded in ${theme}`, async ({ page }) => {
        await start(page, { width: touch ? 390 : 1440, height: 900 }, theme, "synthetic group budgets");
        await page.getByRole("button", { name: "Edit budgets", exact: true }).click();
        await expect(page.getByRole("button", { name: "Edit budgets", exact: true })).toHaveCount(0);
        const card = page.locator(".budget-periods-panel>section.panel").first();
        const menu = card.locator(".budget-period-head .toolbar-actions>details>summary");
        await expect(card.locator(".budget-groups-view")).toBeVisible();
        await expect(card.getByRole("button", { name: "View spending", exact: true })).toBeHidden();
        const headBox = (await card.locator(".budget-period-head").boundingBox())!;
        const menuBox = (await menu.boundingBox())!;
        expect(menuBox.width).toBeGreaterThanOrEqual(44);
        expect(menuBox.height).toBeGreaterThanOrEqual(44);
        expect(Math.abs(menuBox.y-headBox.y)).toBeLessThan(2);
        expect(Math.abs(menuBox.x+menuBox.width-headBox.x-headBox.width)).toBeLessThan(2);
        if (touch) await expect(card.getByText("Selected period", { exact: true })).toBeHidden();
        else await expect(card.getByText("Selected period", { exact: true })).toBeVisible();
        await menu.click();
        await expect(card.getByRole("button", { name: "View spending", exact: true })).toBeVisible();
        await page.keyboard.press("Escape");
        await expect(menu).toBeFocused();
        const group = page.getByRole("region", { name: "No spending group budget", exact: true });
        const header = group.locator(".budget-group-header");
        const summary = group.locator(".budget-group-summary");
        const base = await colorToken(page, "--surface-soft");
        const hover = await colorToken(page, "--interaction-hover");
        await page.mouse.move(0, 0);
        await expect(header).toHaveCSS("background-color", base);
        if (!touch) {
          // Hovering the separate action area must highlight the entire card header.
          await group.getByRole("button", { name: "Add category to No spending group", exact: true }).hover();
          await expect(header).toHaveCSS("background-color", hover);
          await page.mouse.move(0, 0);
          await expect(header).toHaveCSS("background-color", base);
        }
        const activate = async () => touch ? summary.tap() : summary.click();
        await activate();
        await expect(summary).toHaveAttribute("aria-expanded", "true");
        await expect(header).toHaveCSS("background-color", touch ? base : hover);
        await expect(summary).toHaveCSS("background-color", "rgba(0, 0, 0, 0)");
        await page.mouse.move(0, 0);
        await expect(header).toHaveCSS("background-color", base);
        await expect(summary).toHaveCSS("color", await colorToken(page, "--text"));
        await activate();
        await expect(summary).toHaveAttribute("aria-expanded", "false");
        if (!touch) await page.mouse.move(0, 0);
        // The retained button focus and synthetic touch hover must not hold a highlight.
        await expect(header).toHaveCSS("background-color", base);
        if (!touch) {
          await summary.focus();
          await page.keyboard.press("Enter");
          await expect(summary).toHaveAttribute("aria-expanded", "true");
          expect(await summary.evaluate(element => element.matches(":focus-visible"))).toBe(true);
          expect(await summary.evaluate(element => parseFloat(getComputedStyle(element).outlineWidth))).toBeGreaterThan(0);
          await page.keyboard.press("Enter");
          await expect(header).toHaveCSS("background-color", base);
        }
      });
    }
  });
}
