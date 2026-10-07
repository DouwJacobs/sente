import { test, expect, type Page } from '@playwright/test';

const initialPassword = 'synthetic-browser-password';
async function signIn(page: Page, username = 'demo', password = initialPassword) {
  await page.goto('/');
  await page.getByLabel('Username', { exact: true }).fill(username);
  await page.getByLabel('Password', { exact: true }).fill(password);
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Dashboard', exact: true })).toBeVisible();
}
async function settings(page: Page, width: number) {
  if (width === 360) {
    await page.getByRole('navigation', { name: 'Mobile navigation' }).getByRole('button', { name: 'More', exact: true }).click();
    await page.getByRole('navigation', { name: 'More pages' }).getByRole('button', { name: 'Settings', exact: true }).click();
  } else {
    await page.getByRole('navigation', { name: 'Main navigation', exact: true }).getByRole('button', { name: 'Settings', exact: true }).click();
  }
}
async function mutation(page: Page, path: string, body: unknown) {
  const me = await (await page.request.get('/api/me')).json();
  return page.request.post(path, { data: body, headers: { 'X-CSRF-Token': me.csrf, Origin: new URL(page.url()).origin } });
}

for (const width of [1440, 360]) {
  test(`administrator reset and confirmed deletion at ${width}px`, async ({ page, browser }) => {
    await page.setViewportSize({ width, height: 900 });
    await signIn(page);
    const username = `security-${width}`;
    const created = await mutation(page, '/api/users', { username, password: initialPassword, admin: false, budget_member: true });
    expect(created.ok()).toBeTruthy();
    const { id } = await created.json();
    const other = await browser.newContext();
    const otherPage = await other.newPage();
    await signIn(otherPage, username);
    await settings(page, width);
    await page.getByRole('tab', { name: 'Users & access', exact: true }).click();
    await page.getByLabel(`Actions for user ${username}`, { exact: true }).click();
    await page.getByRole('button', { name: 'Reset password', exact: true }).click();
    const reset = page.getByRole('dialog', { name: 'Reset password', exact: true });
    const password = reset.getByLabel('New password', { exact: true });
    const confirmation = reset.getByLabel('Confirm new password', { exact: true });
    await expect(password).toBeFocused();
    await expect(password).not.toHaveAttribute('aria-invalid', 'true');
    await password.fill('short'); await password.blur();
    await expect(password).toHaveAttribute('aria-invalid', 'true');
    await password.fill('synthetic-reset-password');
    await confirmation.fill('different'); await confirmation.blur();
    await expect(confirmation).toHaveAttribute('aria-invalid', 'true');
    await confirmation.fill('synthetic-reset-password');
    await password.fill('synthetic-new-reset-password');
    await expect(confirmation).toHaveAttribute('aria-invalid', 'true');
    await confirmation.fill('synthetic-new-reset-password');
    await reset.getByRole('button', { name: 'Reset password', exact: true }).click();
    await expect(reset).toHaveCount(0);
    expect((await other.request.get('/api/me')).status()).toBe(401);
    await signIn(otherPage, username, 'synthetic-new-reset-password');
    await page.getByLabel(`Actions for user ${username}`, { exact: true }).click();
    await page.getByRole('button', { name: 'Delete user', exact: true }).click();
    const deletion = page.getByRole('dialog', { name: 'Delete user', exact: true });
    await expect(deletion.getByText(/Transactions, rules, imports and historical attribution remain/)).toBeVisible();
    await deletion.getByRole('button', { name: 'Delete user', exact: true }).click();
    await expect(deletion.getByLabel('Confirm username')).toBeFocused();
    await expect(deletion.getByLabel('Confirm username')).toHaveAttribute('aria-invalid', 'true');
    await deletion.getByLabel('Confirm username').fill(username);
    await deletion.getByRole('button', { name: 'Cancel', exact: true }).click();
    expect((await other.request.get('/api/me')).status()).toBe(200);
    await page.getByLabel(`Actions for user ${username}`, { exact: true }).click();
    await page.getByRole('button', { name: 'Delete user', exact: true }).click();
    await deletion.getByLabel('Confirm username').fill(username);
    await deletion.getByRole('button', { name: 'Delete user', exact: true }).click();
    await expect(deletion).toHaveCount(0);
    await expect(page.getByLabel(`Actions for user ${username}`, { exact: true })).toHaveCount(0);
    expect((await other.request.get('/api/me')).status()).toBe(401);
    const deleted = await (await page.request.get(`/api/users?page=0&id=${id}`)).json();
    expect(deleted.items).toHaveLength(0);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
    await other.close();
  });

  test(`self password change validation and session retention at ${width}px`, async ({ page, browser }) => {
    await page.setViewportSize({ width, height: 900 });
    await signIn(page);
    const other = await browser.newContext();
    await signIn(await other.newPage());
    await settings(page, width);
    await page.getByRole('tab', { name: 'Security', exact: true }).click();
    await expect(page.getByText('This session stays signed in.', { exact: false })).toBeVisible();
    const current = page.getByLabel('Current password', { exact: true });
    await current.fill('wrong-password');
    await page.getByLabel('New password', { exact: true }).fill('synthetic-self-password');
    await page.getByLabel('Confirm new password', { exact: true }).fill('synthetic-self-password');
    await page.getByRole('button', { name: 'Change password', exact: true }).click();
    await expect(current).toHaveAttribute('aria-invalid', 'true');
    await expect(page.getByText('Current password is incorrect', { exact: true })).toBeVisible();
    await current.fill(initialPassword);
    await expect(current).not.toHaveAttribute('aria-invalid', 'true');
    await page.getByRole('button', { name: 'Change password', exact: true }).click();
    await expect(current).toHaveValue('');
    expect((await page.request.get('/api/me')).status()).toBe(200);
    expect((await other.request.get('/api/me')).status()).toBe(401);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
    // Restore only this disposable synthetic test account for the following workflows.
    expect((await mutation(page, '/api/password', { old_password: 'synthetic-self-password', new_password: initialPassword })).ok()).toBeTruthy();
    await other.close();
  });
}
