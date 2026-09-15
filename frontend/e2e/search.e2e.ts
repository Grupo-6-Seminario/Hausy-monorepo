import { expect, test, type Page } from '@playwright/test';

// Deliberately fixed UI fixtures; these tests do not claim backend/model quality.
const listing = {
  id: 101,
  rank: 1,
  source: 'zonaprop',
  url: 'https://www.zonaprop.com.ar/101',
  neighborhood: 'Palermo',
  address: 'Humboldt 1900',
  operation: 'alquiler',
  price: { amount: 900, currency: 'USD' },
  expenses: { amount: 110000, currency: 'ARS' },
  rooms: 2,
  bedrooms: 1,
  bathrooms: 1,
  total_area_m2: 50,
  attributes: [
    {
      type: 'natural_light',
      value: 'high',
      provenance: 'stated',
      evidence: 'Muy luminoso.',
    },
    {
      type: 'noise_level',
      value: 'quiet',
      provenance: 'inferred',
      evidence: 'Ubicado al contrafrente.',
    },
  ],
};

async function openSearch(page: Page) {
  await page.goto('/');
  // The server-rendered form can be visible before its handlers are hydrated.
  await page.waitForFunction(() => {
    const canvas = document.querySelector<HTMLCanvasElement>(
      '[data-prompt-luminary]',
    );
    return (
      canvas?.dataset.ready === 'true' || canvas?.dataset.fallback === 'true'
    );
  });
}

async function expectNoOverflow(page: Page) {
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
}

test('welcome, shortlist and return to conversation remain usable', async ({
  page,
}, info) => {
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  await page.route('**/api/agent', (route) =>
    route.fulfill({
      json: {
        reply:
          '## Mi lectura\n**#1 Humboldt 1900** prioriza luz natural.\n\n## Qué falta confirmar\n- El aviso no detalla qué garantías acepta.',
        listings: [listing],
      },
    }),
  );
  await openSearch(page);
  await expect(
    page.getByRole('button', { name: 'Buscar hogares' }),
  ).toBeInViewport();
  await expectNoOverflow(page);
  await page.screenshot({
    path: info.outputPath('welcome.png'),
    animations: 'disabled',
    fullPage: true,
  });
  await page.getByRole('textbox').fill('Dos ambientes con luz en Palermo');
  await page.getByRole('button', { name: 'Buscar hogares' }).click();
  await expect(
    page.getByRole('heading', { name: 'Humboldt 1900', exact: true }),
  ).toBeVisible();
  await expect(page.getByText('Publicado', { exact: true })).toBeVisible();
  await expect(page.getByText('Inferido por Hausy')).toBeVisible();
  await expectNoOverflow(page);
  await page.screenshot({
    path: info.outputPath('results.png'),
    animations: 'disabled',
    fullPage: true,
  });
  await page
    .getByRole('link', { name: 'Seguir la conversación con Hausy' })
    .click();
  await expect(page.getByRole('textbox')).toBeFocused();
  expect(errors).toEqual([]);
});

test('a new turn is readable from its beginning in a long history', async ({
  page,
}) => {
  let turn = 0;
  await page.route('**/api/agent', (route) =>
    route.fulfill({
      json: {
        reply: `## Turno ${++turn}\n${'Esta opción prioriza la luz, pero falta confirmar los requisitos publicados. '.repeat(12)}\n\nFinal del turno ${turn}`,
        listings: [],
      },
    }),
  );
  await openSearch(page);
  for (let i = 1; i <= 3; i++) {
    await page.getByRole('textbox').fill('Priorizar luz natural');
    await page.getByRole('button', { name: 'Buscar hogares' }).click();
    await expect(
      page.getByRole('heading', { name: `Turno ${i}`, exact: true }),
    ).toBeAttached();
  }
  await expect
    .poll(async () => {
      const log = await page.getByRole('log').boundingBox();
      const heading = await page
        .getByRole('heading', { name: 'Turno 3', exact: true })
        .boundingBox();
      return (
        !!log &&
        !!heading &&
        heading.y >= log.y &&
        heading.y + heading.height <= log.y + log.height
      );
    })
    .toBe(true);
  await expect(
    page.getByRole('heading', { name: 'Turno 1', exact: true }),
  ).toBeAttached();
});

test('a failed request restores the query without requiring WebGPU', async ({
  page,
}) => {
  await page.addInitScript(() =>
    Object.defineProperty(navigator, 'gpu', { value: undefined }),
  );
  await page.route('**/api/agent', (route) =>
    route.fulfill({
      status: 502,
      json: { error: 'No pudimos conectarnos con el agente local.' },
    }),
  );
  await openSearch(page);
  await expect(page.locator('canvas')).toHaveAttribute('data-fallback', 'true');
  await page.getByRole('textbox').fill('Busco cerca del subte');
  await page.getByRole('button', { name: 'Buscar hogares' }).click();
  await expect(page.getByRole('alert')).toContainText('No pudimos conectarnos');
  await expect(page.getByRole('textbox')).toHaveValue('Busco cerca del subte');
  await expectNoOverflow(page);
});

test('reduced motion stays usable after resize', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await openSearch(page);
  for (const width of [320, 768, 1180]) {
    await page.setViewportSize({ width, height: 900 });
    await expect(
      page.getByRole('button', { name: 'Buscar hogares' }),
    ).toBeInViewport();
    await expectNoOverflow(page);
  }
});

test('sign in and sign up fit the same visual system', async ({
  page,
}, info) => {
  await page.route('**/api/auth/me', (route) =>
    route.fulfill({ status: 401, json: {} }),
  );
  const hydrated = page.waitForResponse('**/api/auth/me');
  await page.goto('/ingresar');
  await hydrated;
  await expect(page.getByLabel('Email', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Ingresar', exact: true })).toBeInViewport();
  // Exercise the actual keyboard path after the session effect has started.
  await page.getByRole('tab', { name: 'Crear cuenta' }).focus();
  await page.getByRole('tab', { name: 'Crear cuenta' }).press('Enter');
  await expect(page.getByLabel('Nombre', { exact: true })).toBeVisible();
  await expectNoOverflow(page);
  await page.screenshot({
    path: info.outputPath('sign-up.png'),
    animations: 'disabled',
    fullPage: true,
  });
});
