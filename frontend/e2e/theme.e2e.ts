import { expect, test, type Page } from '@playwright/test';

// Which icon shows and which palette paints are both decided in CSS, so they
// only mean anything in a real browser. jsdom cannot see either.

const LIGHT_BACKGROUND = 'rgb(245, 249, 230)';
const DARK_BACKGROUND = 'rgb(16, 34, 24)';

function background(page: Page) {
  return page.evaluate(() => getComputedStyle(document.body).backgroundColor);
}

function icon(page: Page, theme: 'light' | 'dark') {
  return page.locator(`.theme-toggle [data-theme-icon="${theme}"]`);
}

test.describe('with a light system preference', () => {
  test.use({ colorScheme: 'light' });

  test('opens light under a sun and switches to dark under a moon', async ({
    page,
  }) => {
    await page.goto('/');

    await expect(icon(page, 'light')).toBeVisible();
    await expect(icon(page, 'dark')).toBeHidden();
    expect(await background(page)).toBe(LIGHT_BACKGROUND);

    await page.getByRole('button', { name: 'Cambiar al tema oscuro' }).click();

    await expect(icon(page, 'dark')).toBeVisible();
    await expect(icon(page, 'light')).toBeHidden();
    expect(await background(page)).toBe(DARK_BACKGROUND);
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  });

  test('still opens dark on the next visit, and on another page', async ({
    page,
  }) => {
    await page.goto('/');
    await page.getByRole('button', { name: 'Cambiar al tema oscuro' }).click();

    await page.reload();
    expect(await background(page)).toBe(DARK_BACKGROUND);
    await expect(
      page.getByRole('button', { name: 'Cambiar al tema claro' }),
    ).toBeVisible();

    await page.goto('/ingresar');
    expect(await background(page)).toBe(DARK_BACKGROUND);
  });
});

test.describe('with a dark system preference', () => {
  test.use({ colorScheme: 'dark' });

  test('opens dark under a moon and switches to light under a sun', async ({
    page,
  }) => {
    await page.goto('/');

    await expect(icon(page, 'dark')).toBeVisible();
    await expect(icon(page, 'light')).toBeHidden();
    expect(await background(page)).toBe(DARK_BACKGROUND);

    await page.getByRole('button', { name: 'Cambiar al tema claro' }).click();

    await expect(icon(page, 'light')).toBeVisible();
    await expect(icon(page, 'dark')).toBeHidden();
    expect(await background(page)).toBe(LIGHT_BACKGROUND);
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
  });
});

test('applies the remembered choice before the body is parsed', async ({
  page,
}) => {
  // The reason a remembered dark page never flashes light: the document hands
  // the browser its theme in the head, ahead of anything it would paint.
  const html = await (await page.request.get('/')).text();

  expect(html).toContain('hausy-theme');
  expect(html.indexOf('hausy-theme')).toBeLessThan(html.indexOf('<body'));
});
