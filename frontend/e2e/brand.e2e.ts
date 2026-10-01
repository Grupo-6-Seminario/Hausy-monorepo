import { expect, test, type Page } from '@playwright/test';

const realtor = {
  id: '7',
  email: 'marta@inmobiliaria.com',
  name: 'Lounge Propiedades',
  role: 'realtor',
};

// Where the orange mark sits relative to the "ausy" tail, and how visible the tail is.
// Full wordmark: the mark follows the tail. Condensed: it follows the "h".
async function wordmark(page: Page) {
  return page.locator('.site-header .brand').evaluate((brand) => {
    const tail = brand.querySelector('.brand-tail')!.getBoundingClientRect();
    const mark = brand.querySelector('.brand-mark')!.getBoundingClientRect();
    const opacity = getComputedStyle(
      brand.querySelector('.brand-tail')!,
    ).opacity;
    return {
      tailOpacity: Number(opacity),
      markAfterTail: Math.round(mark.left - tail.right),
      markAfterH: Math.round(mark.left - tail.left),
    };
  });
}

async function expectFull(page: Page) {
  await expect
    .poll(() => wordmark(page))
    .toMatchObject({
      tailOpacity: 1,
      markAfterTail: 3,
    });
}

async function expectCondensed(page: Page) {
  await expect
    .poll(() => wordmark(page))
    .toMatchObject({
      tailOpacity: 0,
      markAfterH: 3,
    });
}

// Every header item besides the wordmark, which must stay put while it condenses.
function headerItems(page: Page) {
  return page
    .locator('.site-header > :not(.brand)')
    .evaluateAll((items) =>
      items.map((item) => JSON.stringify(item.getBoundingClientRect())),
    );
}

function twoFrames(page: Page) {
  return page.evaluate(
    () =>
      new Promise((done) =>
        requestAnimationFrame(() => requestAnimationFrame(done)),
      ),
  );
}

async function scrollPastHeading(page: Page) {
  await page.evaluate(() => {
    const heading = document.querySelector('h1')!;
    scrollTo(0, heading.getBoundingClientRect().bottom + scrollY);
  });
}

test('the wordmark condenses past the heading and returns at the top', async ({
  page,
}, info) => {
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  await page.goto('/');
  const before = await headerItems(page);

  await expectFull(page);
  await page.locator('.site-header').screenshot({
    path: info.outputPath('brand-full.png'),
  });

  await scrollPastHeading(page);
  await expectCondensed(page);
  expect(await headerItems(page)).toEqual(before);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.locator('.site-header').screenshot({
    path: info.outputPath('brand-condensed.png'),
  });

  await page.evaluate(() => scrollTo(0, 0));
  await expectFull(page);

  // Reversing mid-transition settles on the latest scroll position.
  await scrollPastHeading(page);
  await page.waitForTimeout(150);
  await page.evaluate(() => scrollTo(0, 0));
  await expectFull(page);
  await scrollPastHeading(page);
  await expectCondensed(page);
  await expect(page.getByRole('link', { name: 'Hausy, inicio' })).toBeVisible();
  expect(errors).toEqual([]);
});

test('reduced motion swaps the wordmark without a transition', async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.goto('/');
  await expectFull(page);
  await scrollPastHeading(page);
  await expect(page.locator('.site-header .brand')).toHaveAttribute(
    'data-condensed',
    '',
  );
  // A transition settles on the frame after it starts, so two frames is instant.
  await twoFrames(page);
  expect(await wordmark(page)).toMatchObject({ tailOpacity: 0, markAfterH: 3 });

  await page.evaluate(() => scrollTo(0, 0));
  await expect(page.locator('.site-header .brand')).not.toHaveAttribute(
    'data-condensed',
  );
  await twoFrames(page);
  expect(await wordmark(page)).toMatchObject({
    tailOpacity: 1,
    markAfterTail: 3,
  });
});

test('sign-in and the agency catalog share the condensing wordmark', async ({
  page,
}, info) => {
  // On a wide screen the sign-in heading never scrolls under the header.
  test.skip(
    info.project.name !== 'mobile',
    'needs a stacked, scrollable layout',
  );
  await page.setViewportSize({ width: 390, height: 360 });
  await page.route('**/api/auth/me', (route) =>
    route.request().headers().referer?.endsWith('/inmobiliaria')
      ? route.fulfill({ json: { user: realtor } })
      : route.fulfill({ status: 401, json: {} }),
  );
  await page.route('**/api/agency/catalog', (route) =>
    route.fulfill({ json: { properties: [] } }),
  );
  for (const path of ['/ingresar', '/inmobiliaria']) {
    await page.goto(path);
    await expect(page.locator('h1')).toBeVisible();
    await expectFull(page);
    await scrollPastHeading(page);
    await expectCondensed(page);
  }
});

test('starting over from a scrolled shortlist settles the wordmark on the new view', async ({
  page,
}) => {
  const attributes = [
    {
      type: 'natural_light',
      value: 'high',
      provenance: 'stated',
      evidence: 'Muy luminoso.',
    },
  ];
  const listings = [1, 2, 3, 4, 5, 6].map((id) => ({
    id,
    rank: id,
    source: 'zonaprop',
    url: `https://www.zonaprop.com.ar/${id}`,
    neighborhood: 'Palermo',
    address: `Humboldt ${id}`,
    operation: 'alquiler',
    price: { amount: 900, currency: 'USD' },
    expenses: { amount: 110000, currency: 'ARS' },
    rooms: 2,
    bedrooms: 1,
    bathrooms: 1,
    total_area_m2: 50,
    attributes,
    matched: attributes,
  }));
  await page.route('**/api/agent', (route) =>
    route.fulfill({ json: { reply: 'Una opción.', listings } }),
  );
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
  await page.getByRole('textbox').fill('Dos ambientes con luz en Palermo');
  await page.getByRole('search').getByRole('button').click();
  await expect(
    page.getByRole('heading', { name: 'Humboldt 1', exact: true }),
  ).toBeVisible();
  await page.evaluate(() => scrollTo({ top: 1200, behavior: 'instant' }));

  // The welcome view replaces the workspace without the window scrolling.
  await page
    .getByRole('button', { name: /Empezar de nuevo/ })
    .evaluate((button: HTMLButtonElement) => button.click());
  await expect(page.locator('h1')).toHaveText(/Primero/);
  const headingHidden = await page.evaluate(
    () =>
      document.querySelector('h1')!.getBoundingClientRect().bottom <=
      document.querySelector('header')!.getBoundingClientRect().bottom,
  );
  expect(headingHidden).toBe(true);
  await expectCondensed(page);
});
