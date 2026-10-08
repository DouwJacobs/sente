import { test, expect } from '@playwright/test';

test('mobile selection cancels on movement and cancellation, supports keyboard, and exits when empty', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');
  await page.getByLabel('Username', { exact: true }).fill('demo');
  await page.getByLabel('Password', { exact: true }).fill('synthetic-browser-password');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await page.getByRole('navigation', { name: 'Mobile navigation', exact: true }).getByRole('button', { name: 'Transactions', exact: true }).click();
  const row = page.locator('.transaction-detail').filter({ hasText: 'Market groceries' });
  await expect(row).toBeVisible();
  const pointer = { pointerId: 1, pointerType: 'touch', button: 0, clientX: 80, clientY: 200 };
  await row.dispatchEvent('pointerdown', pointer);
  await row.dispatchEvent('pointermove', { ...pointer, clientY: 230 });
  await page.waitForTimeout(600); // Beyond the gesture threshold: scrolling must never select.
  await expect(row).not.toHaveAttribute('aria-pressed');
  await row.dispatchEvent('pointercancel', pointer);
  await row.dispatchEvent('pointerdown', pointer);
  await row.dispatchEvent('pointercancel', pointer);
  await page.waitForTimeout(600);
  await expect(row).not.toHaveAttribute('aria-pressed');
  await row.dispatchEvent('pointerdown', pointer);
  await expect(row).toHaveAttribute('aria-pressed', 'true');
  await row.dispatchEvent('pointerup', pointer);
  await row.dispatchEvent('click'); // Browser's click following a hold must be consumed.
  await expect(row).toHaveAttribute('aria-pressed', 'true');
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await row.press('Space');
  await expect(row).not.toHaveAttribute('aria-pressed');
  await row.press('Space');
  await expect(row).toHaveAttribute('aria-pressed', 'true');
  await page.getByLabel('Transaction actions', { exact: true }).click();
  await page.getByRole('button', { name: 'Mark seen (1)', exact: true }).click();
  await expect(row).not.toHaveAttribute('aria-pressed');
  await expect(row.getByRole('img', { name: 'Seen', exact: true })).toBeVisible();
  await row.click();
  await expect(page.getByRole('dialog')).toHaveCount(1);
});

for (const width of [360, 1440]) for (const theme of ['light', 'dark']) {
  test(`categories reuse spending group list ${width} ${theme}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await page.addInitScript(t => localStorage.setItem('finance-theme', t), theme);
    await page.goto('/');
    await page.getByLabel('Username', { exact: true }).fill('demo');
    await page.getByLabel('Password', { exact: true }).fill('synthetic-browser-password');
    await page.getByRole('button', { name: 'Sign in', exact: true }).click();
    if (width === 360) await page.getByRole('button', { name: 'More', exact: true }).click();
    await page.getByRole('button', { name: 'Categories', exact: true }).click();
    const categories = page.locator('.category-list');
    await expect(categories).toBeVisible();
    const categoryStyle = await categories.evaluate(e => ({ columns: getComputedStyle(e).gridTemplateColumns, border: getComputedStyle(e).border, radius: getComputedStyle(e).borderRadius }));
    await page.getByRole('tab', { name: 'Spending groups', exact: true }).click();
    const groups = page.locator('.spending-group-grid');
    await expect(groups).toBeVisible();
    expect(await groups.evaluate(e => ({ columns: getComputedStyle(e).gridTemplateColumns, border: getComputedStyle(e).border, radius: getComputedStyle(e).borderRadius }))).toEqual(categoryStyle);
    await page.getByRole('tab', { name: 'Categories', exact: true }).click();
    await page.getByRole('button', { name: 'Edit Groceries', exact: true }).click();
    await expect(page.getByRole('dialog')).toHaveCount(1);
    await page.keyboard.press('Escape');
    await categories.getByRole('button', { name: /Groceries.*Expense/ }).click();
    await expect(page.getByRole('heading', { name: 'Transactions', exact: true })).toBeVisible();
    await expect(page.locator('.transaction-detail')).toHaveCount(1);
  });
}
