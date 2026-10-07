import { test, expect } from '@playwright/test';
test.use({ screenshot: 'only-on-failure' });
for (const width of [360, 430, 497]) for (const theme of ['light', 'dark']) {
  test(`compact aligned mobile surfaces ${width} ${theme}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 });
    await page.addInitScript(t => localStorage.setItem('finance-theme', t), theme);
    await page.route('**/api/transactions?**', async route => {
      const response = await route.fetch(), body = await response.json();
      body.items = body.items.map((row: Record<string, unknown>) => row.id === 2 ? {
        ...row, description: 'Mobile transfer fixture', spending_group_name: 'Transfer',
        is_transfer: true, household: 0, period_id: null,
      } : row);
      await route.fulfill({ json: body });
    });
    await page.goto('/');
    await page.getByLabel('Username', { exact: true }).fill('demo');
    await page.getByLabel('Password', { exact: true }).fill('synthetic-browser-password');
    await page.getByRole('button', { name: 'Sign in', exact: true }).click();
    const context = page.locator('.context-bar');
    await expect(context).toBeVisible();
    const period = (await context.getByLabel('Budget period', { exact: true }).boundingBox())!;
    const account = (await context.getByLabel('Accounts', { exact: true }).boundingBox())!;
    expect(Math.abs(period.y - account.y)).toBeLessThan(1);
    expect(Math.abs(period.width - account.width)).toBeLessThan(1);
    expect((await context.boundingBox())!.height).toBeLessThan(175);
    const search = page.getByRole('button', { name: 'Search workspace', exact: true });
    await expect(search).toHaveCSS('border-top-width', '0px');
    await expect(search).toHaveCSS('background-color', 'rgba(0, 0, 0, 0)');
    const summary = page.locator('.spending-bucket>summary').first();
    await expect(summary).toBeVisible();
    const inset = await summary.evaluate(e => {
      const outer = e.getBoundingClientRect(), figures = e.querySelector('.budget-actual')!.getBoundingClientRect();
      return { left: figures.left - outer.left, right: outer.right - figures.right };
    });
    expect(inset.left).toBeGreaterThanOrEqual(12);
    expect(inset.right).toBeGreaterThanOrEqual(28);
    await summary.click();
    const category = page.locator('.bucket-category').first();
    const categoryInset = await category.evaluate(e => e.querySelector('.budget-actual')!.getBoundingClientRect().left - e.getBoundingClientRect().left);
    expect(categoryInset).toBeGreaterThanOrEqual(12);
    const nav = page.getByRole('navigation', { name: 'Mobile navigation', exact: true });
    await nav.getByRole('button', { name: 'Transactions', exact: true }).click();
    const transfer = page.locator('.transaction-detail').filter({ hasText: 'Mobile transfer fixture' });
    await expect(transfer).toBeVisible();
    await expect(transfer.getByText('Transfer', { exact: true })).toHaveCount(1);
    expect(await transfer.locator('.row-meta').innerText()).not.toMatch(/\b0\b/);
    await expect(transfer).toContainText('Private');
    const amount = transfer.locator('.transaction-amount');
    const merchant = transfer.locator('.transaction-description>strong');
    const amountBox = (await amount.boundingBox())!, merchantBox = (await merchant.boundingBox())!;
    expect(Math.abs(amountBox.y - merchantBox.y)).toBeLessThan(2);
    await expect(transfer.getByRole('img', { name: 'Accepted', exact: true })).toBeVisible();
    await expect(transfer.getByRole('img', { name: /^(Unseen|Seen)$/ })).toBeVisible();
    expect((await transfer.boundingBox())!.height).toBeLessThan(100);
    await expect(page.locator('.row-check').first()).toBeHidden();
    const logo = transfer.locator('.transaction-logo');
    const logoBox = (await logo.boundingBox())!, rowBox = (await transfer.boundingBox())!;
    expect(logoBox.width).toBe(36);
    expect(Math.abs(logoBox.y + logoBox.height / 2 - rowBox.y - rowBox.height / 2)).toBeLessThan(1);
    await transfer.scrollIntoViewIfNeeded();
    const point = (await transfer.boundingBox())!;
    await page.mouse.move(point.x + 20, point.y + point.height / 2);
    await page.mouse.down();
    await expect(transfer).toHaveAttribute('aria-pressed', 'true');
    await page.mouse.up();
    await expect(page.getByRole('dialog')).toHaveCount(0);
    expect((await transfer.boundingBox())!.y).toBe(point.y);
    await expect(transfer.locator('.transaction-selection-mark')).toBeVisible();
    const another = page.locator('.transaction-detail').filter({ hasText: 'Fuel station' });
    await another.click();
    await expect(another).toHaveAttribute('aria-pressed', 'true');
    await page.getByLabel('Transaction actions', { exact: true }).click();
    await expect(page.getByRole('button', { name: 'Edit selected (2)', exact: true })).toBeVisible();
    await page.keyboard.press('Escape');
    await transfer.click();
    await expect(transfer).toHaveAttribute('aria-pressed', 'false');
    await another.click();
    await expect(another).not.toHaveAttribute('aria-pressed');
    await expect(page.getByText('Done selecting', { exact: true })).toHaveCount(0);
    await transfer.click();
    await expect(page.getByRole('dialog')).toHaveCount(1);
    await page.keyboard.press('Escape');
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  });
}
