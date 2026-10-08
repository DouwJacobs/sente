import { test, expect } from '@playwright/test';
test.use({ screenshot: 'only-on-failure' });

for (const width of [360, 390, 430]) {
  test(`long mobile values ${width}`, {tag:width===360?'@mobile-smoke':[]}, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await page.route('**/api/branding', async route => {
      const response = await route.fetch();
      await route.fulfill({ json: { ...await response.json(), display_name: 'A very long synthetic household name with sixty characters' } });
    });
    await page.route('**/api/transactions?**', async route => {
      const response = await route.fetch(), result = await response.json();
      result.items = result.items.map((row: Record<string, unknown>) => ({
        ...row, amount_cents: row.id === 1 ? 98765432100 : -98765432100,
        account_name: 'Synthetic private everyday spending account with a long name',
        household: false, spending_group_name: 'A long synthetic spending group name',
        merchant_name: 'A synthetic merchant with a very long name and description',
      }));
      await route.fulfill({ json: result });
    });
    await page.goto('/');
    await page.getByLabel('Username', { exact: true }).fill('demo');
    await page.getByLabel('Password', { exact: true }).fill('synthetic-browser-password');
    await page.getByRole('button', { name: 'Sign in', exact: true }).click();
    const nav = page.getByRole('navigation', { name: 'Mobile navigation', exact: true });
    await nav.getByRole('button', { name: 'Transactions', exact: true }).click();
    await expect(page.locator('.transaction-detail')).toHaveCount(5);
    for (const row of await page.locator('.transaction-detail').all()) {
      expect(await row.evaluate(e => e.scrollWidth <= e.clientWidth)).toBe(true);
      await expect(row).toContainText('Private');
      const amount = row.locator('.transaction-amount');
      expect(await amount.evaluate(e => e.scrollWidth <= e.clientWidth)).toBe(true);
    }
    const brand = page.locator('.mobile-brand');
    await expect(brand).toHaveAttribute('title', 'A very long synthetic household name with sixty characters');
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    for (const control of await page.locator('.top-actions button').all()) {
      const box = (await control.boundingBox())!;
      expect(box.width).toBeGreaterThanOrEqual(44);
      expect(box.height).toBeGreaterThanOrEqual(44);
      expect(box.x + box.width).toBeLessThanOrEqual(width);
    }
  });
}

test('mobile non-member navigation presentation', async ({ page }) => {
  // Only presentation is simulated here; server authorization has separate backend coverage.
  await page.setViewportSize({ width: 360, height: 800 });
  await page.route('**/api/me', async route => {
    const response = await route.fetch();
    if (!response.ok()) return route.fulfill({ response });
    const session = await response.json();
    session.user.admin = false;
    session.user.budget_member = false;
    session.user.username = 'A very long synthetic username';
    await route.fulfill({ json: session });
  });
  await page.goto('/');
  await page.getByLabel('Username', { exact: true }).fill('demo');
  await page.getByLabel('Password', { exact: true }).fill('synthetic-browser-password');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  const nav = page.getByRole('navigation', { name: 'Mobile navigation', exact: true });
  await expect(nav.getByRole('button', { name: 'Dashboard', exact: true })).toHaveCount(0);
  await nav.getByRole('button', { name: 'Review', exact: true }).click();
  await expect(page.getByRole('tab', { name: /Needs review/ })).toHaveAttribute('aria-selected', 'true');
  await nav.getByRole('button', { name: 'More', exact: true }).click();
  const more = page.getByRole('navigation', { name: 'More pages' });
  await expect(more.getByRole('button', { name: 'Budgets', exact: true })).toHaveCount(0);
  await expect(more.getByRole('button', { name: 'Accounts', exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});
