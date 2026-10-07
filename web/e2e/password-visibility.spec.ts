import { test, expect, type Page, type Locator } from '@playwright/test';

async function checkPassword(page: Page, scope: Page | Locator, label: string) {
  const input = scope.getByLabel(label, { exact: true });
  const show = scope.getByRole('button', { name: `Show ${label.toLowerCase()}`, exact: true });
  await expect(input).toHaveAttribute('type', 'password');
  await show.hover();
  const hoverStyle = await show.evaluate((button) => {
    const style = getComputedStyle(button);
    return { background: style.backgroundColor, border: style.borderTopColor };
  });
  expect(hoverStyle.background).toBe('rgba(0, 0, 0, 0)');
  expect(hoverStyle.border).toBe('rgba(0, 0, 0, 0)');
  await input.fill('synthetic-visible-password');
  const invalid = await input.getAttribute('aria-invalid');
  await show.click();
  await expect(input).toHaveAttribute('type', 'text');
  await expect(input).toHaveValue('synthetic-visible-password');
  expect(await input.getAttribute('aria-invalid')).toBe(invalid);
  const hide = scope.getByRole('button', { name: `Hide ${label.toLowerCase()}`, exact: true });
  await expect(hide).toHaveAttribute('aria-controls', (await input.getAttribute('id'))!);
  await expect(hide).toHaveAttribute('aria-pressed', 'true');
  await hide.focus();
  await page.keyboard.press('Space');
  await expect(input).toHaveAttribute('type', 'password');
  await show.click();
  await input.fill('');
  await expect(input).toHaveAttribute('type', 'password');
}

for (const width of [1440, 360]) {
  test(`password visibility and single user menu at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    await page.goto('/');
    await checkPassword(page, page, 'Password');
    await page.getByLabel('Username', { exact: true }).fill('demo');
    await page.getByLabel('Password', { exact: true }).fill('synthetic-browser-password');
    await page.getByRole('button', { name: 'Sign in', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Dashboard', exact: true })).toBeVisible();
    if (width === 360) {
      await page.getByRole('navigation', { name: 'Mobile navigation' }).getByRole('button', { name: 'More', exact: true }).click();
      await page.getByRole('navigation', { name: 'More pages' }).getByRole('button', { name: 'Settings', exact: true }).click();
    } else {
      await page.getByRole('navigation', { name: 'Main navigation', exact: true }).getByRole('button', { name: 'Settings', exact: true }).click();
    }
    await page.getByRole('tab', { name: 'Security', exact: true }).click();
    await checkPassword(page, page, 'Current password');
    await checkPassword(page, page, 'New password');
    await checkPassword(page, page, 'Confirm new password');
    await page.getByRole('tab', { name: 'Users & access', exact: true }).click();
    await expect(page.getByLabel('Edit user demo', { exact: true })).toHaveCount(0);
    await page.getByLabel('Actions for user demo', { exact: true }).click();
    await expect(page.getByRole('button', { name: 'Reset password', exact: true })).toHaveCount(0);
    await page.getByRole('button', { name: 'Edit', exact: true }).click();
    const edit = page.getByRole('dialog', { name: 'Edit user', exact: true });
    await expect(edit.getByLabel('Username', { exact: true })).toHaveValue('demo');
    await edit.getByRole('button', { name: 'Close', exact: true }).click();
    await expect(page.getByLabel('Actions for user demo', { exact: true })).toBeFocused();
    await page.getByRole('button', { name: 'Add user', exact: true }).click();
    const add = page.getByRole('dialog', { name: 'Add user', exact: true });
    await checkPassword(page, add, 'Initial password');
    await add.getByRole('button', { name: 'Close', exact: true }).click();
    await page.getByLabel('Actions for user partner', { exact: true }).click();
    await page.getByRole('button', { name: 'Reset password', exact: true }).click();
    const reset = page.getByRole('dialog', { name: 'Reset password', exact: true });
    await checkPassword(page, reset, 'New password');
    await checkPassword(page, reset, 'Confirm new password');
    await reset.getByRole('button', { name: 'Cancel', exact: true }).click();
    await page.getByRole('tab', { name: 'Banking', exact: true }).click();
    await checkPassword(page, page, 'FNB password');
    await page.emulateMedia({ colorScheme: 'dark' });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
    const bounds = await page.getByRole('button', { name: 'Show fnb password', exact: true }).boundingBox();
    expect(bounds!.width).toBeGreaterThanOrEqual(44);
    expect(bounds!.height).toBeGreaterThanOrEqual(44);
  });
}
