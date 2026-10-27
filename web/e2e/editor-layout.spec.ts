import { test, expect } from '@playwright/test';
for (const width of [1440, 900, 390, 360]) test(`split groups and automatic remainder ${width}`, async ({ page }) => {
  await page.setViewportSize({ width, height: 900 });
  await page.addInitScript(theme => localStorage.setItem('finance-theme', theme), width === 390 || width === 900 ? 'dark' : 'light');
  await page.goto('/');
  await page.getByLabel('Username', { exact: true }).fill('demo');
  await page.getByLabel('Password', { exact: true }).fill('synthetic-browser-password');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Dashboard', exact: true })).toBeVisible();
  const me = await (await page.request.get('/api/me')).json();
  const transaction = (await (await page.request.get('/api/transactions?id=5')).json()).items[0];
  expect((await page.request.put('/api/transactions/5', { headers: { 'X-CSRF-Token': me.csrf }, data: {
    version: transaction.version, date: transaction.date, amount_cents: -50000,
    description: transaction.description, assignment: 'auto', is_transfer: false, spending_group_id: 1,
    allocations: [{ category_id: 1, amount_cents: -50000 }],
  } })).ok()).toBe(true);
  await page.getByRole('navigation', { name: width <= 760 ? 'Mobile navigation' : 'Main navigation', exact: true }).getByRole('button', { name: 'Transactions', exact: true }).click();
  await expect(page.getByText('Loading transactions', { exact: true })).toBeHidden();
  const open = () => page.locator('.transaction-detail').filter({ hasText: 'Synthetic long description' }).click();
  await open();
  const editor = page.getByRole('dialog', { name: 'Transaction #5', exact: true });
  const add = editor.getByRole('button', { name: 'Add split', exact: true });
  await expect(editor.getByLabel('Amount', { exact: true })).toHaveCount(0);
  await add.click();
  await editor.getByLabel('Amount 1', { exact: true }).fill('-300');
  await expect(editor.getByLabel('Amount 2', { exact: true })).toHaveValue('-200.00');
  await expect(editor.getByLabel('Amount 2', { exact: true })).toHaveAttribute('readonly', '');
  await editor.getByLabel('Spending group 2', { exact: true }).click();
  const picker = page.getByRole('dialog', { name: 'Select spending group 2', exact: true });
  await picker.getByRole('button', { name: /^Recurring(?: Selected)?$/ }).click();
  await expect(editor.getByLabel('Spending group 2', { exact: true })).toBeFocused();
  await add.click();
  await editor.getByLabel('Amount 2', { exact: true }).fill('-125.45');
  await expect(editor.getByLabel('Amount 3', { exact: true })).toHaveValue('-74.55');
  await editor.getByLabel('Amount 1', { exact: true }).fill('-');
  await expect(editor.getByLabel('Amount 3', { exact: true })).toHaveValue('');
  await editor.getByLabel('Amount 1', { exact: true }).fill('-300');
  await expect(editor.getByLabel('Amount 3', { exact: true })).toHaveValue('-74.55');
  for (const split of await editor.locator('.split-allocation').all()) {
    const group = (await split.locator('.split-group-field button').first().boundingBox())!;
    const category = (await split.getByRole('button', { name: /Category [0-9]/ }).boundingBox())!;
    expect(Math.abs(group.x - category.x)).toBeLessThan(2);
    expect(Math.abs(group.width - category.width)).toBeLessThan(2);
    expect(category.y).toBeGreaterThan(group.y);
    const remove = (await split.getByRole('button', { name: /Remove allocation/ }).boundingBox())!;
    expect(remove.width).toBeGreaterThanOrEqual(44);
    expect(remove.height).toBeGreaterThanOrEqual(44);
  }
  await editor.getByRole('button', { name: 'Save changes', exact: true }).click();
  await expect(editor).toBeHidden();
  const saved = (await (await page.request.get('/api/transactions?id=5')).json()).items[0];
  expect(saved.allocations.map((a: any) => a.amount_cents)).toEqual([-30000, -12545, -7455]);
  expect(saved.allocations[1].spending_group_name).toBe('Recurring');
  expect(saved.allocations[0].spending_group_id).toBe(1);
  await expect(page.getByText('Loading transactions', { exact: true })).toBeHidden();
  await open();
  await expect(editor.getByLabel('Amount 2', { exact: true })).toHaveValue('-125.45');
  await expect(editor.getByLabel('Spending group 2', { exact: true })).toContainText('Recurring');
  await editor.getByRole('button', { name: 'Remove allocation 2', exact: true }).click();
  await expect(editor.getByLabel('Amount 2', { exact: true })).toHaveValue('-200.00');
  await editor.getByRole('button', { name: 'Remove allocation 2', exact: true }).click();
  await expect(editor.getByLabel('Amount', { exact: true })).toHaveCount(0);
  expect(await editor.evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await editor.getByRole('button', { name: 'Save changes', exact: true }).click();
  await expect(editor).toBeHidden();
});
