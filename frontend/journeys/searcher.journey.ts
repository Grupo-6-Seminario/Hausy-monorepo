import { expect, request, test } from '@playwright/test';

import { hydrated, listingA, ports, realtor, search } from './fixture';

// The realtor sees contacts on their catalog; reading it straight from the API
// checks that the click reached Postgres, not just that the UI reacted. The
// realtor signs in once, before the steps: sign-in hashes a password, which
// would swamp the contact step's timing.
async function contactCounter(): Promise<() => Promise<number>> {
  const api = await request.newContext({
    baseURL: `http://127.0.0.1:${ports.api}`,
  });
  const signIn = await api.post('/api/auth/sign-in', { data: realtor });
  expect(signIn.ok()).toBe(true);
  const { token } = await signIn.json();
  return async () => {
    const catalog = await api.get('/api/agency/catalog', {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(catalog.ok()).toBe(true);
    const { properties } = await catalog.json();
    const listing = properties.find(
      (p: { url: string }) => p.url === listingA.url,
    );
    expect(listing).toBeDefined();
    return listing.contact_count;
  };
}

test('New searcher answers the amenities question and contacts a listing', async ({
  page,
}) => {
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  const contactCount = await contactCounter();
  const before = await contactCount();

  await test.step('Open the search', async () => {
    await page.goto('/');
    await hydrated(page);
  });

  await test.step('Search with amenities', async () => {
    await search(
      page,
      /¿Qué estás buscando\?/,
      'Busco alquiler en Palermo con amenities',
    );
    await expect(
      page.getByRole('heading', { name: /Cuando dijiste «con amenities»/ }),
    ).toBeVisible();
  });

  const card = page.getByRole('article').filter({ hasText: listingA.address });
  await test.step('Answer the clarification', async () => {
    await page.getByLabel('Pileta').check();
    await page.getByRole('button', { name: 'Continuar búsqueda' }).click();
    await expect(card).toBeVisible();
  });

  await test.step('Contact the listing', async () => {
    const publication = page.waitForEvent('popup');
    await card.getByRole('link', { name: /^Contactar/ }).click();
    await expect(await publication).toHaveTitle('Publicación 60023997');
    await expect.poll(contactCount, { intervals: [50] }).toBe(before + 1);
  });

  expect(errors).toEqual([]);
});

test('Registered searcher declares a qualification and finds what accepts it', async ({
  page,
}) => {
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));

  await test.step('Sign up', async () => {
    await page.goto('/ingresar');
    await page.getByRole('tab', { name: 'Crear cuenta' }).click();
    await page.getByLabel('Nombre').fill('Journeys');
    await page.getByLabel('Email').fill(`buscador-${Date.now()}@journeys.test`);
    await page.getByLabel('Contraseña').fill('journeys-searcher');
    await page.getByRole('button', { name: 'Crear cuenta' }).click();
    await page
      .getByRole('link', { name: 'Completar mi perfil y buscar' })
      .click();
    await hydrated(page);
  });

  await test.step('Declare the qualification', async () => {
    // The inputs are hidden behind chip labels, so click the labels.
    const answer = (fact: string, choice: string) =>
      page.getByRole('group', { name: fact }).getByText(choice).click();
    await answer('Garantía', 'Garantía propietaria');
    await answer('¿Podés comprobar tus ingresos?', 'Sí');
    await answer('Mascotas', 'Ninguna');
    await page
      .getByLabel('Ingresos mensuales')
      .selectOption({ label: 'Más de $3.000.000' });
    await expect(page.getByLabel('Garantía propietaria')).toBeChecked();
    const use = page.getByRole('button', { name: 'Usar estos datos' });
    await use.click();
    // The panel closes once the answers are saved and applied.
    await expect(use).toBeHidden();
  });

  await test.step('Search where the qualification is accepted', async () => {
    await search(page, /¿Qué estás buscando\?/, 'Busco alquiler en Palermo');
    // The eligible card's own line, which "Calificás con condiciones" lacks.
    await expect(
      page.getByRole('article').filter({ hasText: 'Soler 5818' }),
    ).toContainText('Cumplís lo que pide');
  });

  await test.step('Open the listings that refuse it', async () => {
    await search(page, /Sumá a tu búsqueda/, 'Busco alquiler en Monserrat');
    const refused = page.getByRole('button', {
      name: 'Ver las 3 propiedades que no aceptan tu situación',
    });
    await expect(refused).toBeVisible();
    // Every Monserrat listing refuses propietaria, so none is shown yet.
    await expect(page.getByRole('article')).toHaveCount(0);
    await refused.click();
    await expect(
      page.getByRole('article').filter({ hasText: 'Moreno al 400' }),
    ).toBeVisible();
  });

  await test.step('Reload and keep the session', async () => {
    await page.reload();
    // Saved answers load only for a signed-in searcher.
    await expect(page.getByLabel('Garantía propietaria')).toBeChecked();
    await expect(page.getByLabel('Ingresos mensuales')).toHaveValue('3000000-');
  });

  expect(errors).toEqual([]);
});
