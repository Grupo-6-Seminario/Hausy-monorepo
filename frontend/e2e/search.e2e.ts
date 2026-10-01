import { expect, test, type Page } from '@playwright/test';

// Deliberately fixed UI fixtures; these tests do not claim backend/model quality.
const attributes = [
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
];
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
  attributes,
  matched: attributes,
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
    page.getByRole('search').getByRole('button'),
  ).toBeInViewport();
  await expectNoOverflow(page);
  await page.screenshot({
    path: info.outputPath('welcome.png'),
    animations: 'disabled',
    fullPage: true,
  });
  await page.getByRole('textbox').fill('Dos ambientes con luz y silencio en Palermo');
  await page.getByRole('search').getByRole('button').click();
  await expect(
    page.getByRole('heading', { name: 'Humboldt 1900', exact: true }),
  ).toBeVisible();
  const matched = page.getByRole('region', {
    name: 'Coincide con lo que pediste',
  });
  await expect(matched.getByText('Publicado', { exact: true })).toBeVisible();
  await expect(matched.getByText('Inferido', { exact: true })).toBeVisible();
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

test('clarification survives reload and resumes the same search', async ({ page }, info) => {
  const question = {
    id: 'rooms-question', request: 'Busco dos habitaciones en Palermo',
    source: 'dos habitaciones', prompt: 'Cuando dijiste «dos habitaciones», ¿ambientes o dormitorios?',
    kind: 'search', choices: [{ id: 'ambientes', label: 'Dos ambientes' }, { id: 'dormitorios', label: 'Dos dormitorios' }],
  };
  let sessionID = '';
  await page.route('**/api/agent**', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ json: { clarification: question } });
      return;
    }
    const body = JSON.parse(route.request().postData() || '{}');
    expect(body.session_id).toBeTruthy();
    if (!body.answer) {
      sessionID = body.session_id;
      await route.fulfill({ json: { clarification: question } });
    } else {
      expect(body.session_id).toBe(sessionID);
      expect(body.answer).toEqual({ question_id: 'rooms-question', selected: ['dormitorios'] });
      await route.fulfill({ json: { reply: 'Una propiedad con dos dormitorios.', listings: [listing] } });
    }
  });
  await openSearch(page);
  await page.getByRole('textbox', { name: '¿Qué estás buscando?' }).fill(question.request);
  await page.getByRole('search').getByRole('button').click();
  await expect(page.getByRole('heading', { name: question.prompt })).toBeVisible();
  await expect(page.locator('form.query-form')).toHaveCount(0);
  await page.screenshot({ path: info.outputPath('clarification.png'), animations: 'disabled', fullPage: true });

  await page.reload();
  await expect(page.getByRole('heading', { name: question.prompt })).toBeVisible();
  await page.getByLabel('Dos dormitorios').check();
  await page.getByRole('button', { name: 'Continuar búsqueda' }).click();
  await expect(page.getByRole('heading', { name: 'Humboldt 1900', exact: true })).toBeVisible();
  await page.screenshot({ path: info.outputPath('clarified-results.png'), animations: 'disabled', fullPage: true });
});

test('Contactar records one intent and still opens the publication', async ({
  page,
}, info) => {
  await page.route('**/api/agent', (route) =>
    route.fulfill({ json: { reply: 'Una opción.', listings: [listing] } }),
  );

  const intents: { url: string; body: unknown }[] = [];
  await page.route('**/api/listings/*/contact-intents', async (route) => {
    const body = JSON.parse(route.request().postData() ?? '{}');
    intents.push({ url: route.request().url(), body });
    await route.fulfill({
      status: 201,
      json: { intent_id: body.intent_id, listing_id: '101', recorded: true },
    });
  });

  await openSearch(page);
  await page.getByRole('textbox').fill('Dos ambientes con luz en Palermo');
  await page.getByRole('search').getByRole('button').click();

  const contact = page.getByRole('link', { name: /Contactar/ });
  await expect(contact).toBeVisible();
  await expect(contact).toHaveAttribute('href', listing.url);
  await page
    .locator('.property-card-footer')
    .first()
    .screenshot({
      path: info.outputPath('contact-action.png'),
      animations: 'disabled',
    });

  // The link opens the publication in a new tab; the intent goes out alongside.
  const [publication] = await Promise.all([
    page.context().waitForEvent('page'),
    contact.click(),
  ]);
  await publication.close();

  await expect.poll(() => intents.length).toBe(1);
  expect(new URL(intents[0].url).pathname).toBe(
    '/api/listings/101/contact-intents',
  );
  expect(Object.keys(intents[0].body as object).sort()).toEqual([
    'intent_id',
    'source',
  ]);
  expect((intents[0].body as { source: string }).source).toBe(
    'search_result_card',
  );
  await expect(page.locator('.listing-contact-note')).toHaveText('');
  await expectNoOverflow(page);
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
    await page.getByRole('search').getByRole('button').click();
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
  await page.getByRole('search').getByRole('button').click();
  await expect(page.getByRole('alert')).toContainText('No pudimos conectarnos');
  await expect(page.getByRole('textbox')).toHaveValue('Busco cerca del subte');
  await expectNoOverflow(page);
});

// The view-transition layers that animate during the next 1.5 s, by name.
function morphLayers(page: Page) {
  return page.evaluate(
    () =>
      new Promise<string[]>((resolve) => {
        const seen = new Set<string>();
        const started = performance.now();
        const look = () => {
          for (const animation of document.getAnimations()) {
            const layer = (animation.effect as KeyframeEffect | null)
              ?.pseudoElement;
            if (layer?.startsWith('::view-transition')) seen.add(layer);
          }
          if (performance.now() - started < 1500) requestAnimationFrame(look);
          else resolve([...seen]);
        };
        requestAnimationFrame(look);
      }),
  );
}

test('sending glides the composer and the message into the workspace', async ({
  page,
}) => {
  await page.route('**/api/agent', async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 1200));
    await route.fulfill({
      json: { reply: 'Una opción.', listings: [listing] },
    });
  });
  await openSearch(page);
  await page.getByRole('textbox').fill('Dos ambientes con luz en Palermo');

  const layers = morphLayers(page);
  await page.getByRole('search').getByRole('button').click();

  expect(await layers).toEqual(
    expect.arrayContaining([
      '::view-transition-group(composer)',
      '::view-transition-group(sent-message)',
      '::view-transition-new(results)',
    ]),
  );
  await expect(
    page.getByRole('heading', { name: 'Humboldt 1900', exact: true }),
  ).toBeVisible();
  await expect(page.locator('html')).not.toHaveAttribute('data-morphing');
});

test('reduced motion sends without animating the view', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.route('**/api/agent', async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 600));
    await route.fulfill({
      json: { reply: 'Una opción.', listings: [listing] },
    });
  });
  await openSearch(page);
  await page.getByRole('textbox').fill('Dos ambientes con luz en Palermo');

  const layers = morphLayers(page);
  await page.getByRole('search').getByRole('button').click();

  expect(await layers).toEqual([]);
  await expect(
    page.getByRole('heading', { name: 'Humboldt 1900', exact: true }),
  ).toBeVisible();
});

test('reduced motion stays usable after resize', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await openSearch(page);
  for (const width of [320, 768, 1180]) {
    await page.setViewportSize({ width, height: 900 });
    await expect(
      page.getByRole('search').getByRole('button'),
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
  await expect(
    page.getByRole('button', { name: 'Ingresar', exact: true }),
  ).toBeInViewport();
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

test('eligibility sections, the zero-results line and the declared qualification', async ({
  page,
}, info) => {
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  const bodies: { qualification?: Record<string, string[]> }[] = [];
  await page.route('**/api/agent', async (route) => {
    bodies.push(JSON.parse(route.request().postData() ?? '{}'));
    await route.fulfill({
      json: {
        reply: '## Mi lectura\n**#1 Humboldt 1900** acepta tu garantía.',
        listings: [
          { ...listing, rank: 1, eligibility: { state: 'eligible' } },
          {
            ...listing,
            id: 102,
            rank: 2,
            url: 'https://www.zonaprop.com.ar/102',
            address: 'Gorriti 4800',
            eligibility: {
              state: 'conditionally_eligible',
              conditions: [
                {
                  reason: 'discretionary',
                  rule: {
                    fact: 'guarantee',
                    evidence:
                      'Garantía CABA o seguro de caución (ver cuáles permite la propietaria)',
                  },
                },
              ],
            },
          },
        ],
        relaxations: [{ fact: 'guarantee', value: 'caucion', count: 14 }],
      },
    });
  });

  await openSearch(page);
  // The micro-interview opens with the page.
  await page.getByLabel('Garantía propietaria').check();
  await page.getByRole('button', { name: 'Usar estos datos' }).click();
  await page.getByRole('textbox').fill('Alquiler en Palermo');
  await page.getByRole('search').getByRole('button').click();

  await expect(
    page.getByRole('heading', { name: 'Calificás' }),
  ).toBeVisible();
  await expect(
    page.getByRole('heading', { name: 'Depende de la inmobiliaria' }),
  ).toBeVisible();
  await expect(
    page.getByText(/ver cuáles permite la propietaria/),
  ).toBeVisible();
  await expect(
    page.getByText('Si conseguís seguro de caución, vuelven 14 propiedades.'),
  ).toBeVisible();
  await expect(
    page.getByText('garantía propietaria', { exact: true }),
  ).toBeVisible();
  expect(bodies.at(-1)?.qualification).toEqual({ guarantee: ['propietaria'] });
  await expectNoOverflow(page);
  await page.screenshot({
    path: info.outputPath('eligibility.png'),
    animations: 'disabled',
    fullPage: true,
  });
  expect(errors).toEqual([]);
});
