import { test, expect } from '@playwright/test';
for (const width of [360, 430, 1440]) for (const theme of ['light', 'dark']) {
 test(`compact workspace controls ${width} ${theme}`, async ({ page }) => {
  await page.setViewportSize({ width, height: 850 });
  await page.addInitScript(t => localStorage.setItem('finance-theme', t), theme);
  await page.goto('/');
  await page.getByLabel('Username', { exact: true }).fill('demo');
  await page.getByLabel('Password', { exact: true }).fill('synthetic-browser-password');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  const scope = page.locator('.dashboard-scope');
  await expect(scope).toBeVisible();
  await expect(scope).toHaveCSS('border-top-width', '0px');
  expect((await scope.boundingBox())!.height).toBeLessThanOrEqual(width < 760 ? 120 : 70);
  const totals = page.getByRole('region', { name: 'Period totals', exact: true });
  expect((await totals.boundingBox())!.y).toBeLessThanOrEqual(width < 760 ? 265 : 240);
  for (const control of await scope.locator('select,button').all()) {
   const box = (await control.boundingBox())!;
   expect(box.height).toBeGreaterThanOrEqual(44);
  }
  const nav = page.getByRole('navigation', { name: width < 760 ? 'Mobile navigation' : 'Main navigation', exact: true });
  await nav.getByRole('button', { name: 'Transactions', exact: true }).click();
  const filters = page.getByRole('region', { name: 'Transaction filters', exact: true });
  const button = filters.getByRole('button', { name: /^Filters/ });
  await expect(button).toHaveAttribute('aria-expanded', 'false');
  await expect(filters.getByLabel('Accounts', { exact: true })).toBeHidden();
  await expect(filters.getByLabel('Search transactions', { exact: true })).toBeVisible();
  expect((await filters.boundingBox())!.height).toBeLessThan(90);
  const first = page.locator('.transaction-detail').first();
  await expect(first).toBeVisible();
  expect((await first.boundingBox())!.y).toBeLessThan(width < 760 ? 420 : 360);
  await button.click();
  await filters.getByLabel('Accounts', { exact: true }).selectOption('1');
  await filters.getByLabel('Budget period', { exact: true }).selectOption('1');
  await filters.getByLabel('Filter by category', { exact: true }).selectOption('1');
  await page.keyboard.press('Escape');
  await expect(button).toBeFocused();
  await expect(button).toHaveAttribute('aria-expanded', 'false');
  await expect(filters.getByLabel('Accounts', { exact: true })).toBeHidden();
  await expect(filters.getByLabel('Search transactions', { exact: true })).toHaveValue('');
  await expect(page.locator('.transaction-scope-summary')).toContainText('Everyday account · October 2026');
  await expect(page.locator('.transaction-detail')).toHaveCount(1);
  await filters.getByLabel('Search transactions', { exact: true }).fill('Market');
  await expect(page.locator('.transaction-detail')).toHaveCount(1);
  await filters.getByRole('button', { name: 'Clear filters', exact: true }).click();
  await expect(filters.getByLabel('Search transactions', { exact: true })).toHaveValue('');
  await expect(page.locator('.transaction-detail')).toHaveCount(5);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
 });
}
