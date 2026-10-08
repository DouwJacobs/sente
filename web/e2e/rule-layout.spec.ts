import { desktopPhoneCases } from './coverage-cases';
import { test, expect } from '@playwright/test';
for (const {width,theme} of desktopPhoneCases) {
 test(`compact rule indicators ${width} ${theme}`, async ({ page }) => {
  await page.setViewportSize({ width, height: 850 });
  await page.addInitScript(t => localStorage.setItem('finance-theme', t), theme);
  const pattern = 'Synthetic merchant description match with a long reference and service name';
  await page.route('**/api/rules?**', route => route.fulfill({ json: {
   items: [{ id: 9001, account_id: 1, account_name: 'Everyday account', pattern, category_id: 1, category_name: 'Groceries', direction: 'debit', priority: 0, enabled: 0, version: 1, builtin: 0 },
    { id: 9002, account_id: 0, pattern: 'Global synthetic rule', category_id: 1, category_name: 'Groceries', direction: 'any', priority: 0, enabled: 1, version: 1, builtin: 1 }], total: 2, list_version: 'synthetic'
  }}));
  await page.route('**/api/merchant-rules?**', route => route.fulfill({ json: {
   items: [{ id: 9001, merchant_name: 'Synthetic merchant with a long readable name', merchant_logo: '', pattern, account_id: 0, account_name: 'All accounts', direction: 'credit', priority: 2, category_name: 'Groceries', spending_group_name: 'Recurring', enabled: 1, version: 1 }], total: 1, list_version: 'synthetic'
  }}));
  await page.goto('/');
  await page.getByLabel('Username', { exact: true }).fill('demo');
  await page.getByLabel('Password', { exact: true }).fill('synthetic-browser-password');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  if (width < 760) await page.getByRole('button', { name: 'More', exact: true }).click();
  await page.getByRole('button', { name: 'Categories', exact: true }).click();
  await expect(page.getByRole('img', { name: 'Expense', exact: true }).first()).toBeVisible();
  await page.getByRole('tab', { name: 'Automatic rules', exact: true }).click();
  const row = page.locator('.rule-row').filter({ hasText: pattern });
  await expect(row.getByRole('img', { name: 'Paused', exact: true })).toBeVisible();
  await expect(page.getByRole('img', { name: 'Global fallback', exact: true })).toBeVisible();
  await expect(row.getByRole('button')).toHaveCount(0); // Row actions are inside the closed menu.
  await row.locator('summary').click();
  await expect(row.getByRole('button', { name: 'Resume', exact: true })).toBeVisible();
  await expect(row.getByRole('button', { name: 'Delete rule '+pattern, exact: true })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(row.locator('summary')).toBeFocused();
  await page.getByRole('tab', { name: 'Merchant rules', exact: true }).click();
  const merchant = page.locator('.merchant-rule-row');
  await expect(merchant).toContainText('Groceries');
  await expect(merchant).toContainText('Recurring');
  await expect(merchant).not.toContainText(pattern);
  await expect(merchant).not.toContainText('All accounts');
  await expect(merchant.getByRole('img', { name: 'Active', exact: true })).toBeVisible();
  await expect(merchant.getByRole('img', { name: 'Money in', exact: true })).toHaveCount(0);
  await merchant.locator('summary').click();
  await expect(merchant.getByRole('button', { name: 'Edit', exact: true })).toBeVisible();
  await expect(merchant.getByRole('button', { name: 'Preview unnamed transactions', exact: true })).toBeVisible();
  await page.keyboard.press('Escape');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
 });
}
