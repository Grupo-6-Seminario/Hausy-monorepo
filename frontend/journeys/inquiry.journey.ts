import { expect, test } from '@playwright/test';

import { hydrated, ports, realtor, search } from './fixture';

test('A published property accepts the chat qualification and delivers the consented inquiry', async ({
  page,
  browser,
}, testInfo) => {
  const run = Date.now();
  const examples = [
    {
      name: 'Caución',
      guarantee: 'Seguro de caución',
      hardness: 'hard',
      badge: 'Calificás',
    },
    {
      name: 'A criterio',
      guarantee: 'Seguro de caución',
      hardness: 'discretionary',
      badge: 'Depende de la inmobiliaria',
    },
    {
      name: 'Sin requisitos',
      guarantee: '',
      hardness: 'hard',
      badge: 'A confirmar',
    },
    {
      name: 'Propietaria',
      guarantee: 'Garantía propietaria',
      hardness: 'hard',
      badge: '',
    },
  ];
  await page.goto('/ingresar');
  await page.getByLabel('Email').fill(realtor.email);
  await page.getByLabel('Contraseña').fill(realtor.password);
  await page.getByRole('button', { name: 'Ingresar' }).click();
  await page.getByRole('link', { name: 'Ir al catálogo' }).click();

  for (const example of examples) {
    await test.step(`Publish ${example.name}`, async () => {
      await page.getByRole('button', { name: 'Agregar propiedad' }).click();
      const editor = page.getByRole('dialog', { name: 'Agregar propiedad' });
      await editor
        .getByLabel('URL de la publicación')
        .fill(
          `http://127.0.0.1:${ports.model}/publicacion/${run}-${example.name.replaceAll(' ', '-')}`,
        );
      await editor
        .getByLabel('Dirección', { exact: true })
        .fill(`Consulta ${run} ${example.name}`);
      await editor.getByLabel('Barrio', { exact: true }).fill('palermo');
      await editor
        .getByLabel('Descripción', { exact: true })
        .fill('Dos ambientes con balcón, publicados por la inmobiliaria.');
      await editor.getByLabel('Precio', { exact: true }).fill('700000');
      if (example.guarantee) {
        const guarantees = editor.getByRole('group', {
          name: 'Garantías aceptadas',
        });
        await guarantees.getByText(example.guarantee, { exact: true }).click();
        await guarantees.getByLabel('Exigencia').selectOption(example.hardness);
      }
      await editor.getByRole('button', { name: 'Guardar propiedad' }).click();
      await expect(editor).toBeHidden();
      await expect(
        page.getByRole('heading', {
          name: `Consulta ${run} ${example.name}`,
          exact: true,
        }),
      ).toBeVisible();
    });
  }

  const context = await browser.newContext({
    baseURL: `http://127.0.0.1:${ports.frontend}`,
  });
  try {
    const searcher = await context.newPage();
    await searcher.goto('/');
    await hydrated(searcher);
    await search(
      searcher,
      /¿Qué estás buscando\?/,
      'Busco alquiler en Palermo y tengo seguro de caución',
    );
    for (const example of examples) {
      const card = searcher
        .getByRole('article')
        .filter({ hasText: `Consulta ${run} ${example.name}` });
      if (!example.badge) {
        await expect(card).toHaveCount(0);
        continue;
      }
      await expect(card).toBeVisible();
      await expect(card.locator('.eligibility-badge')).toContainText(
        example.badge,
      );
    }
    const discretionary = searcher
      .getByRole('article')
      .filter({ hasText: `Consulta ${run} A criterio` });
    await expect(
      discretionary.getByRole('heading', {
        name: 'Lo que pide la inmobiliaria',
      }),
    ).toBeVisible();
    await expect(discretionary.locator('q')).toHaveCount(0);
    const card = searcher
      .getByRole('article')
      .filter({ hasText: `Consulta ${run} Caución` });
    await card.getByRole('button', { name: 'Enviar consulta' }).click();
    const dialog = searcher.getByRole('dialog', {
      name: `Consulta ${run} Caución`,
    });
    await expect(
      dialog.getByText('Declaraste: Seguro de caución'),
    ).toBeVisible();
    await expect(dialog.getByText('Cumplís', { exact: true })).toBeVisible();
    await dialog.getByRole('button', { name: 'Cancelar', exact: true }).focus();
    for (let i = 0; i < 8; i++) {
      await searcher.keyboard.press('Shift+Tab');
      // A native modal permits browser chrome, but no background page control.
      await expect
        .poll(() =>
          searcher.evaluate(
            () =>
              document.activeElement === document.body ||
              Boolean(document.activeElement?.closest('dialog')),
          ),
        )
        .toBe(true);
    }
    await dialog.getByLabel('Tu nombre').fill('Ana Journeys');
    await dialog.getByLabel('Email', { exact: true }).fill('ana@journeys.test');
    await dialog
      .getByLabel('Mensaje (opcional)')
      .fill('¿Podemos coordinar una visita?');
    await dialog.screenshot({
      path: testInfo.outputPath('inquiry-preview.png'),
    });
    await dialog
      .getByRole('button', { name: 'Enviar consulta', exact: true })
      .click();
    await expect(dialog.getByRole('status')).toContainText(
      'tu consulta le llegó',
    );
    await dialog.getByRole('button', { name: 'Cerrar', exact: true }).click();
    await page.reload();
    const received = page
      .getByRole('region', { name: 'Consultas' })
      .getByRole('listitem')
      .filter({
        has: page.getByRole('heading', {
          name: `Consulta ${run} Caución`,
          exact: true,
        }),
      });
    await expect(received).toContainText('Ana Journeys');
    await expect(received).toContainText('Declaró: Seguro de caución');
    await expect(received).toContainText('¿Podemos coordinar una visita?');
    await expect(
      received.getByRole('link', { name: 'ana@journeys.test' }),
    ).toHaveAttribute('href', 'mailto:ana@journeys.test');
    await received.screenshot({
      path: testInfo.outputPath('agency-receipt.png'),
    });
  } finally {
    await context.close();
  }
});
