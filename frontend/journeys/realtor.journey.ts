import { expect, test } from '@playwright/test';

import { listingB, realtor } from './fixture';

test('Realtor edits a catalog listing and the change persists', async ({
  page,
}) => {
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  // A new value every run, so a rerun without a fresh seed still proves a save.
  const address = `Rivadavia al 1800, edición ${Date.now()}`;
  const row = page
    .getByRole('listitem')
    .filter({ has: page.locator(`a[href="${listingB.url}"]`) });

  await test.step('Sign in', async () => {
    await page.goto('/ingresar');
    await page.getByLabel('Email').fill(realtor.email);
    await page.getByLabel('Contraseña').fill(realtor.password);
    await page.getByRole('button', { name: 'Ingresar' }).click();
    await page.getByRole('link', { name: 'Ir al catálogo' }).click();
  });

  await test.step('Open the catalog', async () => {
    await expect(
      page.getByRole('heading', { name: 'Tu catálogo' }),
    ).toBeVisible();
    await expect(row).toBeVisible();
  });

  await test.step('Edit the listing', async () => {
    await row.getByRole('button', { name: 'Editar' }).click();
    const editor = page.getByRole('dialog', { name: 'Editar propiedad' });
    await editor.getByLabel('Dirección').fill(address);
    await editor.getByRole('button', { name: 'Guardar cambios' }).click();
    await expect(editor).toBeHidden();
    await expect(row.getByRole('heading')).toHaveText(address);
  });

  await test.step('Reload and see the change', async () => {
    await page.reload();
    await expect(row.getByRole('heading')).toHaveText(address);
  });

  expect(errors).toEqual([]);
});
