import { expect, test, type Page } from '@playwright/test';

const realtor = {
  id: '7',
  email: 'marta@inmobiliaria.com',
  name: 'Lounge Propiedades',
  role: 'realtor',
};

const properties = [
  {
    id: '14',
    agency: 'Lounge Propiedades',
    source: 'agency',
    contact_count: 7,
    url: 'https://www.zonaprop.com.ar/propiedades/palermo-101.html',
    neighborhood: 'palermo',
    address: 'Humboldt al 1900',
    description: 'Semipiso luminoso al contrafrente.',
    operation: 'alquiler',
    price: { amount: 850, currency: 'USD' },
    expenses: { amount: 120000, currency: 'ARS' },
    rooms: 3,
    bedrooms: 2,
    bathrooms: 1,
    total_area_m2: 65,
  },
  {
    id: '15',
    agency: 'Lounge Propiedades',
    source: 'agency',
    contact_count: 6,
    url: 'https://www.zonaprop.com.ar/propiedades/belgrano-202.html',
    neighborhood: 'belgrano',
    address: 'Mendoza al 2400',
    description: 'Departamento de dos ambientes.',
    operation: 'alquiler',
    price: { amount: 720000, currency: 'ARS' },
    expenses: { amount: null, currency: 'ARS' },
    rooms: 2,
    bedrooms: 1,
    bathrooms: 1,
    total_area_m2: 48,
  },
];

async function expectNoOverflow(page: Page) {
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
}

test('a realtor can inspect the catalog and open the property editor', async ({
  page,
}, info) => {
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  await page.route('**/api/auth/me', (route) =>
    route.fulfill({ json: { user: realtor } }),
  );
  await page.route('**/api/agency/catalog', (route) =>
    route.fulfill({ json: { properties } }),
  );

  await page.goto('/inmobiliaria');

  await expect(
    page.getByRole('heading', { name: 'Tu catálogo' }),
  ).toBeVisible();
  await expect(page.getByText('2 propiedades activas')).toBeVisible();
  await expect(page.getByText('13 contactos iniciados')).toBeVisible();
  await expectNoOverflow(page);
  await page.screenshot({
    path: info.outputPath('agency-catalog.png'),
    animations: 'disabled',
    fullPage: true,
  });

  await page.getByRole('button', { name: 'Editar' }).first().click();
  await expect(
    page.getByRole('dialog', { name: 'Editar propiedad' }),
  ).toBeVisible();
  await expect(page.getByLabel('Dirección')).toHaveValue('Humboldt al 1900');
  await expectNoOverflow(page);
  await page.screenshot({
    path: info.outputPath('agency-editor.png'),
    animations: 'disabled',
    fullPage: true,
  });
  expect(errors).toEqual([]);
});
