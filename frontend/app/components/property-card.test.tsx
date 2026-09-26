import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Listing } from '@/lib/types';
import { PropertyCard } from './property-card';

const sampleListing: Listing = {
  id: 101,
  source: 'zonaprop',
  url: 'https://www.zonaprop.com.ar/propiedades/departamento-palermo-101.html',
  neighborhood: 'palermo',
  agency: 'LOUNGE PROPIEDADES S.A.',
  address: 'Humboldt al 1900, Palermo',
  description:
    'Excelente semipiso al contrafrente con mucha luz natural y vista abierta.',
  operation: 'alquiler',
  price: { amount: 850, currency: 'USD' },
  expenses: { amount: 120000, currency: 'ARS' },
  total_area_m2: 65,
  covered_area_m2: 60,
  rooms: 3,
  bedrooms: 2,
  bathrooms: 1,
  floor: '7',
  attributes: [
    {
      type: 'natural_light',
      value: 'high',
      provenance: 'stated',
      evidence: 'mucha luz natural',
    },
    {
      type: 'noise_level',
      value: 'quiet',
      provenance: 'inferred',
      evidence: 'al contrafrente con vista abierta',
    },
    {
      type: 'exposure',
      value: 'contrafrente',
      provenance: 'stated',
    },
  ],
};
sampleListing.matched = sampleListing.attributes;

describe('PropertyCard', () => {
  // "Sin datos de requisitos" was shown even when the ad publishes its
  // requirements and only the searcher's own data is missing.
  it.each([
    [[], 'No publica requisitos'],
    [[{ reason: 'missing', rule: { fact: 'guarantee', values: ['caucion'], evidence: 'Sistema FINAER' } }], 'Publica requisitos · completá tus datos'],
    [[{ reason: 'unverifiable', rule: { fact: 'income_band', values: ['3'], evidence: 'Ingresos 3 veces el alquiler' } }], 'Requisito no verificable'],
  ])('names why eligibility is unknown (%#)', (conditions, label) => {
    render(<PropertyCard listing={{ ...sampleListing, eligibility: { state: 'unknown', conditions } }} />);
    expect(screen.getByText(label)).toBeVisible();
  });

  it('renders property price, expenses, and neighborhood', () => {
    render(<PropertyCard listing={sampleListing} />);

    expect(screen.getByText(/USD 850/i)).toBeVisible();
    expect(screen.getByText(/120\.000/i)).toBeVisible();
    expect(screen.getAllByText(/palermo/i)[0]).toBeVisible();
    expect(screen.getByText(/Humboldt al 1900/i)).toBeVisible();
  });

  it('renders rooms, bedrooms, bathrooms, and floor metrics', () => {
    render(<PropertyCard listing={sampleListing} />);

    expect(screen.getByText(/3 amb/i)).toBeVisible();
    expect(screen.getByText(/2 dorm/i)).toBeVisible();
    expect(screen.getByText(/1 baño/i)).toBeVisible();
    expect(screen.getByText(/65 m²/i)).toBeVisible();
  });

  it('explicitly warns when expenses are not published rather than showing zero', () => {
    const listingWithoutExpenses: Listing = {
      ...sampleListing,
      expenses: { amount: null, currency: null },
    };

    render(<PropertyCard listing={listingWithoutExpenses} />);

    expect(screen.getByText(/expensas no publicadas/i)).toBeVisible();
  });

  it('renders attributes distinguishing between stated and inferred provenance', () => {
    render(<PropertyCard listing={sampleListing} />);

    // Stated attribute
    const statedBadge = screen.getByText(/luz natural: alta/i);
    expect(statedBadge).toBeVisible();

    // Inferred attribute with trust indicator
    const inferredBadge = screen.getByText(/silencioso/i);
    expect(inferredBadge).toBeVisible();

    // Evidence quote displayed
    expect(screen.getByText(/mucha luz natural/i)).toBeVisible();
  });

  it('gives stated and inferred attributes distinct provenance hooks', () => {
    render(<PropertyCard listing={sampleListing} />);

    const statedItem = screen
      .getByText(/luz natural: alta/i)
      .closest('li') as HTMLLIElement;
    const inferredItem = screen
      .getByText(/silencioso/i)
      .closest('li') as HTMLLIElement;

    expect(statedItem).toHaveClass('is-published');
    expect(inferredItem).toHaveClass('is-inferred');
    expect(statedItem.className).not.toBe(inferredItem.className);
  });

  it('labels the inferred attribute with the unknown signal, the stated one without it', () => {
    render(<PropertyCard listing={sampleListing} />);

    expect(screen.getByText('Inferido por Hausy')).toHaveClass(
      'evidence-provenance',
    );
    expect(screen.getAllByText('Publicado')[0]).not.toHaveClass(
      'evidence-provenance',
    );
  });

  it('resolves the two provenance hooks to different border styles', () => {
    const css = readFileSync(
      path.join(
        path.dirname(fileURLToPath(import.meta.url)),
        '..',
        'globals.css',
      ),
      'utf8',
    );

    const rule = (selector: string) => {
      const at = css.indexOf(selector);
      expect(at, `${selector} is missing from globals.css`).toBeGreaterThan(-1);
      return css.slice(at, css.indexOf('}', at));
    };

    const published = rule('.property-evidence > ul > li.is-published');
    const inferred = rule('.property-evidence > ul > li.is-inferred');

    // Solid vs dashed carries the distinction without relying on hue, so it
    // survives greyscale and a colorblind viewer.
    expect(published).toMatch(
      /border-left:\s*2px solid var\(--signal-evidence\)/,
    );
    expect(inferred).toMatch(
      /border-left:\s*2px dashed var\(--signal-unknown\)/,
    );
  });

  it('provides a link to the original listing source', () => {
    render(<PropertyCard listing={sampleListing} />);

    const link = screen.getByRole('link', { name: /ver en zonaprop/i });
    expect(link).toHaveAttribute(
      'href',
      'https://www.zonaprop.com.ar/propiedades/departamento-palermo-101.html',
    );
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('marks a property only when Hausy explicitly recommends it', () => {
    const { rerender } = render(
      <PropertyCard listing={{ ...sampleListing, rank: 4 }} isRecommended />,
    );

    expect(screen.getByText('Destacada por Hausy')).toBeVisible();

    rerender(<PropertyCard listing={{ ...sampleListing, rank: 1 }} />);
    expect(screen.queryByText('Destacada por Hausy')).toBeNull();
  });
});

describe('PropertyCard contact intent', () => {
  const UUID_V4 =
    /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  function recordingBackend(status = 201) {
    return vi.fn(async (_path: string, init: RequestInit) => {
      const sent = JSON.parse(init.body as string) as { intent_id: string };
      return new Response(
        JSON.stringify({
          intent_id: sent.intent_id,
          listing_id: '101',
          recorded: true,
        }),
        { status, headers: { 'Content-Type': 'application/json' } },
      );
    });
  }

  const contactControl = () => screen.getByRole('link', { name: /contactar/i });

  // A polite live region, mounted before it has anything to say so the message
  // is announced. It deliberately avoids role="status": the search workspace
  // owns the page's single status announcement.
  const contactNote = () => {
    const note = document.querySelector('.listing-contact-note');
    expect(note).not.toBeNull();
    expect(note).toHaveAttribute('aria-live', 'polite');
    return note as HTMLElement;
  };

  it('offers a visible, keyboard-reachable Contactar control on the card', async () => {
    vi.stubGlobal('fetch', recordingBackend());
    render(<PropertyCard listing={sampleListing} />);

    const contact = contactControl();
    expect(contact).toBeVisible();
    expect(contact).toHaveAttribute('href', sampleListing.url);
    expect(contact).toHaveAttribute('rel', 'noopener noreferrer');

    // Reachable by keyboard alone, and Enter activates it like a click.
    await userEvent.tab();
    expect(contact).toHaveFocus();
  });

  it('sends exactly {intent_id, source} to the listing contact-intents path', async () => {
    const fetchMock = recordingBackend();
    vi.stubGlobal('fetch', fetchMock);
    render(<PropertyCard listing={sampleListing} />);

    await userEvent.click(contactControl());

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const [requestPath, init] = fetchMock.mock.calls[0];
    expect(requestPath).toBe('/api/listings/101/contact-intents');
    expect(init.method).toBe('POST');

    const body = JSON.parse(init.body as string);
    expect(Object.keys(body).sort()).toEqual(['intent_id', 'source']);
    expect(body.source).toBe('search_result_card');
    expect(body.intent_id).toMatch(UUID_V4);
  });

  it('emits one event per deliberate activation, each with a new intent id', async () => {
    const fetchMock = recordingBackend();
    vi.stubGlobal('fetch', fetchMock);
    render(<PropertyCard listing={sampleListing} />);

    await userEvent.click(contactControl());
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    await userEvent.click(contactControl());
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));

    const ids = fetchMock.mock.calls.map(
      ([, init]) => JSON.parse(init.body as string).intent_id,
    );
    expect(new Set(ids).size).toBe(2);

    // The card never keeps its own tally: the backend owns the aggregate.
    expect(screen.queryByText(/\b[12] interesad/i)).toBeNull();
  });

  it('keeps the contact path open but emits nothing when the listing has no stable id', async () => {
    const fetchMock = recordingBackend();
    vi.stubGlobal('fetch', fetchMock);
    const { id: _id, ...untraceable } = sampleListing;
    render(<PropertyCard listing={untraceable} />);

    const contact = contactControl();
    expect(contact).toHaveAttribute('href', sampleListing.url);
    expect(contact).toHaveAttribute('data-contact-tracking', 'unavailable');

    await userEvent.click(contact);

    expect(fetchMock).not.toHaveBeenCalled();
    await waitFor(() =>
      expect(contactNote()).toHaveTextContent(
        /no pudimos registrar tu interés/i,
      ),
    );
  });

  it('never derives the listing id from the publication URL', async () => {
    const fetchMock = recordingBackend();
    vi.stubGlobal('fetch', fetchMock);
    render(
      <PropertyCard
        listing={{
          ...sampleListing,
          id: 'zonaprop-77',
          url: 'https://example.com/9999',
        }}
      />,
    );

    await userEvent.click(contactControl());

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    expect(fetchMock.mock.calls[0][0]).toBe(
      '/api/listings/zonaprop-77/contact-intents',
    );
  });

  it('surfaces a lightweight notice when tracking fails, without blocking contact', async () => {
    const fetchMock = vi.fn().mockRejectedValue(new Error('offline'));
    vi.stubGlobal('fetch', fetchMock);
    render(<PropertyCard listing={sampleListing} />);

    const contact = contactControl();
    await userEvent.click(contact);

    await waitFor(() =>
      expect(contactNote()).toHaveTextContent(
        /no pudimos registrar tu interés/i,
      ),
    );
    // The searcher is not trapped: no dialog, no disabled control, and the link
    // to the publication is untouched.
    expect(screen.queryByRole('alertdialog')).toBeNull();
    expect(contact).toHaveAttribute('href', sampleListing.url);
    expect(contact).not.toHaveAttribute('aria-disabled');
  });

  it('keeps the contact control inside the cozy-green palette and pill shape', () => {
    const css = readFileSync(
      path.join(
        path.dirname(fileURLToPath(import.meta.url)),
        '..',
        'globals.css',
      ),
      'utf8',
    );

    const rule = (selector: string) => {
      const at = css.indexOf(selector);
      expect(at, `${selector} is missing from globals.css`).toBeGreaterThan(-1);
      return css.slice(at, css.indexOf('}', at));
    };

    const contact = rule('.property-card-footer .listing-contact');
    expect(contact).toMatch(/background:\s*var\(--primary\)/);
    expect(contact).toMatch(/color:\s*var\(--primary-foreground\)/);
    // No literal colors sneak in beside the tokens.
    expect(contact).not.toMatch(/#[0-9a-f]{3,8}\b/i);

    // Shape comes from the shared footer-action rule: one soft pill system.
    expect(rule('.property-card-footer a')).toMatch(
      /border-radius:\s*var\(--radius-pill\)/,
    );
  });

  it('clears the failure notice once a later activation is recorded', async () => {
    const fetchMock = vi
      .fn()
      .mockRejectedValueOnce(new Error('offline'))
      .mockImplementation(recordingBackend());
    vi.stubGlobal('fetch', fetchMock);
    render(<PropertyCard listing={sampleListing} />);

    await userEvent.click(contactControl());
    await waitFor(() =>
      expect(contactNote()).toHaveTextContent(/no pudimos registrar/i),
    );

    await userEvent.click(contactControl());
    await waitFor(() => expect(contactNote()).toHaveTextContent(''));
  });
});

describe('PropertyCard eligibility', () => {
  it('carries its eligibility badge and the listing’s condition inside the card', () => {
    render(
      <PropertyCard
        listing={{
          ...sampleListing,
          rank: 2,
          eligibility: {
            state: 'conditionally_eligible',
            conditions: [
              {
                reason: 'discretionary',
                rule: {
                  fact: 'guarantee',
                  evidence: 'ver cuáles permite la propietaria',
                },
              },
            ],
          },
        }}
      />,
    );
    const badge = screen.getByText('Depende de la inmobiliaria');
    const condition = screen.getByText(/ver cuáles permite la propietaria/);
    expect(badge.closest('article')).not.toBeNull();
    expect(condition.closest('article')).toBe(badge.closest('article'));
  });
});

describe('PropertyCard qualities', () => {
  // Seen live 2026-09-26: the searcher picked two amenities and the card listed
  // all ten qualities Hausy had parsed from the ad.
  it('shows only the qualities that answer the search', () => {
    const pool = { type: 'amenity', value: 'pileta', provenance: 'stated', evidence: 'PILETA' } as const;
    const gym = { type: 'amenity', value: 'gimnasio', provenance: 'stated', evidence: 'GYM' } as const;
    render(
      <PropertyCard
        listing={{
          ...sampleListing,
          attributes: [
            { type: 'amenity', value: 'seguridad', provenance: 'stated', evidence: 'SEGURIDAD' },
            pool,
            { type: 'outdoor_space', value: 'balcon', provenance: 'stated', evidence: 'SALIDA A BALCON' },
            gym,
          ],
          matched: [pool, gym],
        }}
      />,
    );

    const qualities = screen.getByRole('region', { name: 'Coincide con lo que pediste' });
    const items = within(qualities).getAllByRole('listitem');
    expect(items).toHaveLength(2);
    expect(items[0]).toHaveTextContent('Pileta');
    expect(items[1]).toHaveTextContent('Gimnasio');
    expect(screen.queryByText('Balcón')).toBeNull();
    expect(screen.queryByText('Seguridad')).toBeNull();
  });

  // Seen live 2026-09-26: an ad names its amenities in one sentence, and the
  // card quoted that sentence once per amenity.
  it('quotes a sentence once when it names several of the asked qualities', () => {
    const sentence = 'Piscina, SUM, Parrilla, Gimnasio';
    render(
      <PropertyCard
        listing={{
          ...sampleListing,
          matched: [
            { type: 'amenity', value: 'gimnasio', provenance: 'stated', evidence: sentence },
            { type: 'amenity', value: 'pileta', provenance: 'stated', evidence: sentence },
          ],
        }}
      />,
    );

    const items = within(screen.getByRole('region', { name: 'Coincide con lo que pediste' })).getAllByRole('listitem');
    expect(items).toHaveLength(1);
    expect(items[0]).toHaveTextContent('Gimnasio · Pileta');
    expect(screen.getAllByText(sentence)).toHaveLength(1);
  });

  it('shows no qualities when the search asked for none', () => {
    render(<PropertyCard listing={{ ...sampleListing, matched: undefined }} />);

    expect(screen.queryByRole('region', { name: 'Coincide con lo que pediste' })).toBeNull();
    expect(screen.queryByText(/luz natural/i)).toBeNull();
  });
});
