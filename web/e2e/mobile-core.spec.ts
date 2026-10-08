import { test, expect, type Page, type Locator } from '@playwright/test';

test.use({ screenshot: 'only-on-failure' });

async function signIn(page: Page) {
  await page.goto('/');
  await page.getByLabel('Username', { exact: true }).fill('demo');
  await page.getByLabel('Password', { exact: true }).fill('synthetic-browser-password');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Dashboard', exact: true })).toBeVisible();
}

async function fits(page: Page, target: Locator, label: string, touch = false) {
  const box = await target.boundingBox();
  expect(box, label).not.toBeNull();
  const width = page.viewportSize()!.width;
  expect(box!.x, `${label}: left edge`).toBeGreaterThanOrEqual(-1);
  expect(box!.x + box!.width, `${label}: right edge`).toBeLessThanOrEqual(width + 1);
  if (touch) {
    expect(box!.width, `${label}: touch width`).toBeGreaterThanOrEqual(44);
    expect(box!.height, `${label}: touch height`).toBeGreaterThanOrEqual(44);
  }
}

for (const [width, height] of [[360, 800], [640, 360]]) {
  for (const theme of ['light', 'dark']) {
    test(`mobile core ${width}x${height} ${theme}`, {tag:width===360&&theme==='light'?'@mobile-smoke':[]}, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.addInitScript(t => localStorage.setItem('finance-theme', t), theme);
      const errors: string[] = [];
      page.on('pageerror', error => errors.push(error.message));
      await signIn(page);
      const nav = page.getByRole('navigation', { name: 'Mobile navigation', exact: true });
      for (const button of await nav.getByRole('button').all()) await fits(page, button, 'navigation', true);
      const more = nav.getByRole('button', { name: 'More', exact: true });
      await more.click();
      const pages = page.getByRole('navigation', { name: 'More pages' });
      await expect(pages.getByRole('button', { name: 'Accounts', exact: true })).toBeFocused();
      const panelBox = (await pages.boundingBox())!, navBox = (await nav.boundingBox())!;
      expect(panelBox.y + panelBox.height).toBeLessThanOrEqual(navBox.y);
      await page.keyboard.press('Escape');
      await expect(pages).toBeHidden();
      await expect(more).toBeFocused();
      await more.click();
      await page.getByRole('heading', { name: 'Dashboard', exact: true }).click();
      await expect(pages).toBeHidden();
      await more.click();
      await pages.getByRole('button', { name: 'Accounts', exact: true }).click();
      await expect(page.getByRole('heading', { name: 'Accounts', exact: true })).toBeVisible();
      await expect(more).toHaveClass('active');
      await nav.getByRole('button', { name: 'Transactions', exact: true }).click();
      await page.getByLabel('Search transactions', { exact: true }).fill('No matches for this filter');
      await expect(page.locator('.transaction-detail')).toHaveCount(0);
      await nav.getByRole('button', { name: 'Review', exact: true }).click();
      await expect(page.getByLabel('Search transactions', { exact: true })).toHaveValue('');
      await expect(nav.getByRole('button', { name: 'Review', exact: true })).toHaveAttribute('aria-current', 'page');
      await expect(page.getByRole('tab', { name: /Needs review/ })).toHaveAttribute('aria-selected', 'true');
      await expect(page.locator('.transaction-detail')).toHaveCount(1);
      await nav.getByRole('button', { name: 'Transactions', exact: true }).click();
      await expect(page.getByRole('tab', { name: 'All transactions', exact: true })).toHaveAttribute('aria-selected', 'true');
      await page.getByLabel('Transaction actions', { exact: true }).click();
      await page.getByRole('button', { name: 'Select transactions', exact: true }).click();
      const selection = page.locator('.transaction-detail').filter({ hasText: 'Market groceries' });
      await fits(page, selection, 'selection', true);
      await selection.click();
      await expect(page.getByRole('dialog')).toHaveCount(0);
      await expect(selection).toHaveAttribute('aria-pressed', 'true');
      await page.getByLabel('Transaction actions', { exact: true }).click();
      await expect(page.getByRole('button', { name: 'Edit selected (1)', exact: true })).toBeVisible();
      await page.getByRole('button', { name: 'Cancel selection', exact: true }).click();
      await page.getByRole('button', { name: /^Filters/ }).click();
      const account = (await page.getByLabel('Accounts', { exact: true }).boundingBox())!;
      const period = (await page.getByLabel('Budget period', { exact: true }).boundingBox())!;
      expect(period.y).toBeGreaterThanOrEqual(account.y + account.height);
      await expect(page.locator('.transaction-head .check')).toBeHidden();
      await page.getByRole('button', { name: /^Filters/ }).click();
      for (const amount of await page.locator('.transaction-amount').all()) await fits(page, amount, 'amount');
      const row = page.locator('.transaction-detail').filter({ hasText: 'Synthetic long description' });
      await row.click();
      const editor = page.getByRole('dialog', { name: 'Transaction #5', exact: true });
      await editor.getByLabel('Description', { exact: true }).fill('Synthetic long description with a preserved mobile draft');
      await editor.getByRole('button', { name: 'Add split', exact: true }).click();
      await editor.getByLabel('Amount', { exact: true }).nth(0).fill('-750.00');
      await editor.getByLabel('Amount', { exact: true }).nth(1).fill('-400.00');
      for (const field of await editor.getByLabel('Amount', { exact: true }).all()) await fits(page, field, 'split amount');
      for (const button of await editor.getByRole('button', { name: /Remove allocation/ }).all()) await fits(page, button, 'remove split', true);
      const category = editor.getByLabel('Category 1', { exact: true });
      await category.click();
      const nested = page.getByRole('dialog').last();
      await expect(nested).not.toHaveAttribute('aria-label', 'Transaction #5');
      await page.keyboard.press('Escape');
      await expect(category).toBeFocused();
      await expect(editor.getByLabel('Description', { exact: true })).toHaveValue('Synthetic long description with a preserved mobile draft');
      await editor.getByRole('button', { name: 'Save changes', exact: true }).click();
      await expect(editor.getByLabel('Amount', { exact: true }).nth(1)).toHaveAttribute('aria-invalid', 'true');
      await expect(editor.getByLabel('Amount', { exact: true }).nth(1)).toBeFocused();
      await editor.getByLabel('Amount', { exact: true }).nth(1).fill('-500.00');
      await expect(editor.getByLabel('Amount', { exact: true }).nth(1)).not.toHaveAttribute('aria-invalid', 'true');
      const save = editor.getByRole('button', { name: 'Save changes', exact: true });
      await save.scrollIntoViewIfNeeded();
      await fits(page, save, 'save', true);
      const saveBox = (await save.boundingBox())!;
      expect(saveBox.y + saveBox.height).toBeLessThanOrEqual(height);
      expect(await editor.evaluate(e => e.scrollWidth <= e.clientWidth)).toBe(true);
      await save.click();
      await expect(editor).toBeHidden();
      const me = await (await page.request.get('/api/me')).json();
      const saved = (await (await page.request.get('/api/transactions?id=5')).json()).items[0];
      expect(saved.allocations.reduce((sum: number, a: { amount_cents: number }) => sum + a.amount_cents, 0)).toBe(saved.amount_cents);
      // Restore the shared synthetic row for the next viewport, through the normal authorized API.
      await page.request.put('/api/transactions/5', { headers: { 'X-CSRF-Token': me.csrf }, data: {
        version: saved.version, date: saved.date, amount_cents: saved.amount_cents,
        description: 'Synthetic long description for a purchase that needs categorization and review on mobile',
        assignment: 'auto', is_transfer: false, allocations: [{ category_id: null, amount_cents: saved.amount_cents }],
      }});
      await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
      const last = (await page.locator('.pagination, .transaction-row').last().boundingBox())!;
      expect(last.y + last.height).toBeLessThanOrEqual((await nav.boundingBox())!.y + 1);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      expect(errors).toEqual([]);
    });
  }
}

for (const width of [390, 430, 699, 700, 701, 760, 761, 1024, 1440]) {
  test(`navigation and editor breakpoint ${width}`, {tag:width===1440?'@mobile-smoke':[]}, async ({ page }) => {
    await page.setViewportSize({ width, height: 720 });
    await signIn(page);
    const nav = page.getByRole('navigation', { name: width <= 760 ? 'Mobile navigation' : 'Main navigation', exact: true });
    await nav.getByRole('button', { name: 'Transactions', exact: true }).click();
    await page.locator('.transaction-detail').filter({ hasText: 'Market groceries' }).click();
    const editor = page.getByRole('dialog', { name: 'Transaction #2', exact: true });
    await fits(page, editor.getByLabel('Signed amount (ZAR)', { exact: true }), 'amount field');
    expect(await editor.evaluate(e => e.scrollWidth <= e.clientWidth)).toBe(true);
    await editor.getByRole('button', { name: 'Close', exact: true }).last().click();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  });
}
