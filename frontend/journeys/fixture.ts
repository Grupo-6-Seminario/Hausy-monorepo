import type { Page } from '@playwright/test';

// What e2e/journeys/seed loads and fakemodel serves. Keep the two in sync.
export const ports = {
  model: 8710,
  api: 8711,
  frontend: 8712,
  inspector: 8713,
};

export const realtor = {
  email: 'inmobiliaria@journeys.test',
  password: 'journeys-realtor',
};

// Listing A: Palermo, with pileta, owned by the realtor.
export const listingA = {
  url: `http://127.0.0.1:${ports.model}/publicacion/60023997`,
  address: 'Cerviño al 4700',
};

// Listing B: the realtor's own Congreso entry, never searched.
export const listingB = {
  url: `http://127.0.0.1:${ports.model}/publicacion/inmobiliaria-congreso`,
};

export async function hydrated(page: Page) {
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

export async function search(page: Page, label: RegExp, text: string) {
  const box = page.getByLabel(label);
  await box.fill(text);
  await box.press('Enter');
}
