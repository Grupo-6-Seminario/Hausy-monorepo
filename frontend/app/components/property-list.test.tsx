import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { withFacts } from '@/lib/eligibility-facts.fixture';
import type { Listing } from '@/lib/types';
import { PropertyList } from './property-list';

afterEach(() => vi.unstubAllGlobals());

const mockListings: Listing[] = [
  {
    id: 1,
    source: 'zonaprop',
    url: 'https://zonaprop.com.ar/1',
    neighborhood: 'palermo',
    address: 'Humboldt 1900',
    description: 'Depto luminoso',
    price: { amount: 800, currency: 'USD' },
    expenses: { amount: 100000, currency: 'ARS' },
    rooms: 2,
    bedrooms: 1,
    bathrooms: 1,
    attributes: [
      { type: 'natural_light', value: 'high', provenance: 'stated' },
    ],
  },
  {
    id: 2,
    source: 'zonaprop',
    url: 'https://zonaprop.com.ar/2',
    neighborhood: 'belgrano',
    address: 'Cabildo 2000',
    description: 'Semipiso silencioso',
    price: { amount: 950, currency: 'USD' },
    expenses: { amount: null, currency: null },
    rooms: 3,
    bedrooms: 2,
    bathrooms: 2,
    attributes: [
      { type: 'noise_level', value: 'quiet', provenance: 'inferred' },
    ],
  },
];

describe('PropertyList', () => {
  it('renders a list of property cards with count', () => {
    render(<PropertyList listings={mockListings} />);

    expect(
      screen.getByRole('region', { name: /propiedades encontradas/i }),
    ).toBeVisible();
    expect(screen.getByText(/2 propiedades seleccionadas/i)).toBeVisible();
    expect(screen.getByText(/Humboldt 1900/i)).toBeVisible();
    expect(screen.getByText(/Cabildo 2000/i)).toBeVisible();
    expect(screen.getByText('#1')).toBeVisible();
    expect(screen.getByText('#2')).toBeVisible();
  });

  it('renders an empty state when no listings are found', () => {
    render(<PropertyList listings={[]} />);

    expect(
      screen.getByText(/no encontramos propiedades que coincidan exactamente/i),
    ).toBeVisible();
    // The writer never suggests another neighborhood; the screen must not either.
    expect(
      screen.getByText(/el presupuesto, los ambientes o algún requisito/),
    ).toBeVisible();
    expect(screen.queryByText(/barrios/)).toBeNull();
    expect(screen.queryByText(/no te mostramos/)).toBeNull();
  });

  // Seen in the spec 006 verification: a search whose only listing asked for
  // caución showed the plain empty state, without the line that brings it back.
  it('names the listings eligibility hid and how to bring them back when none is shown', async () => {
    vi.stubGlobal(
      'fetch',
      withFacts(() => Promise.reject(new Error('unreachable'))),
    );
    render(
      <PropertyList
        listings={[]}
        hiddenIneligible={1}
        relaxations={[{ fact: 'guarantee', value: 'caucion', count: 1 }]}
      />,
    );

    expect(
      screen.getByText(
        'Hay 1 propiedad que no te mostramos porque no cumplís sus requisitos.',
      ),
    ).toBeVisible();
    expect(
      await screen.findByText(
        'Si conseguís seguro de caución, vuelve 1 propiedad.',
      ),
    ).toBeVisible();
  });

  // A dog or an income below the line hides listings that no guarantee brings
  // back: the screen still needs a way forward.
  it('offers a next step when hidden listings have no relaxation', () => {
    render(<PropertyList listings={[]} hiddenIneligible={2} />);

    expect(
      screen.getByText(
        'Hay 2 propiedades que no te mostramos porque no cumplís sus requisitos.',
      ),
    ).toBeVisible();
    expect(
      screen.getByText(
        'Revisá tu situación o contale a Hausy qué requisito podés flexibilizar.',
      ),
    ).toBeVisible();
  });

  it('renders a loading skeleton when isLoading is true', () => {
    render(<PropertyList listings={[]} isLoading={true} />);

    expect(screen.getByLabelText(/buscando propiedades/i)).toBeVisible();
  });
});

describe('PropertyList eligibility sections', () => {
  const withState = (
    listing: Listing,
    rank: number,
    eligibility: Listing['eligibility'],
  ): Listing => ({ ...listing, rank, eligibility });

  it('groups listings by eligibility, names the condition and keeps ranks continuous', () => {
    render(
      <PropertyList
        listings={[
          withState(mockListings[0], 1, { state: 'eligible' }),
          withState(mockListings[1], 2, {
            state: 'conditionally_eligible',
            conditions: [
              {
                reason: 'discretionary',
                rule: {
                  fact: 'guarantee',
                  evidence:
                    'Garantía CABA o caución (ver cuáles permite la propietaria)',
                },
              },
            ],
          }),
          withState(
            { ...mockListings[0], url: 'u3', address: 'Gorriti 4000' },
            3,
            { state: 'unknown' },
          ),
        ]}
      />,
    );

    const headings = screen
      .getAllByRole('heading', { level: 3 })
      .map((h) => h.textContent);
    expect(headings).toEqual([
      'Calificás',
      'Calificás con condiciones',
      'A confirmar',
    ]);
    expect(screen.getByText(/ver cuáles permite la propietaria/)).toBeVisible();
    expect(screen.getByText('#3')).toBeVisible();
    expect(
      screen.getByText('Ordenadas por si podés alquilarlas'),
    ).toBeVisible();
  });
});

// Spec 006, error 4: the zero-results line names each instrument as the
// catalog does, including the guarantor's pay slips.
it('names each instrument in the zero-results line from the catalog', async () => {
  vi.stubGlobal(
    'fetch',
    withFacts(() => Promise.reject(new Error('unreachable'))),
  );
  render(
    <PropertyList
      listings={[mockListings[0]]}
      relaxations={[
        { fact: 'guarantee', value: 'caucion', count: 2 },
        { fact: 'guarantee', value: 'recibos_garante', count: 1 },
      ]}
    />,
  );
  expect(
    await screen.findByText(/Si conseguís seguro de caución, vuelven 2 propiedades/),
  ).toBeVisible();
  expect(
    screen.getByText(
      /Si conseguís recibos de sueldo de un garante, vuelve 1 propiedad\./,
    ),
  ).toBeVisible();
});

it('separates evidenced light matches from alternatives without labeling every card', () => {
  render(<PropertyList listings={[
    { ...mockListings[0], rank: 1, qualitative_fit: 'exact' },
    { ...mockListings[1], rank: 2, qualitative_fit: 'unconfirmed' },
  ]} />);
  expect(screen.getByRole('heading', { name: 'Coincidencias con evidencia' })).toBeVisible();
  expect(screen.getByRole('heading', { name: 'Otras opciones por confirmar' })).toBeVisible();
  expect(screen.getByText('#1')).toBeVisible();
  expect(screen.getByText('#2')).toBeVisible();
});

it('keeps a branch without a qualitative requirement visible beside another branch with one', () => {
  render(<PropertyList listings={[
    { ...mockListings[0], rank: 1, qualitative_fit: 'exact' },
    { ...mockListings[1], rank: 2 },
  ]} />);
  expect(screen.getByText(/Humboldt 1900/i)).toBeVisible();
  expect(screen.getByText(/Cabildo 2000/i)).toBeVisible();
});

it('gives each card the reason for its verdict from what the searcher declared', async () => {
  vi.stubGlobal(
    'fetch',
    withFacts(() => Promise.reject(new Error('unreachable'))),
  );
  render(
    <PropertyList
      listings={[
        {
          ...mockListings[0],
          rank: 1,
          eligibility: {
            state: 'eligible',
            met: [{ fact: 'guarantee', values: ['caucion'] }],
          },
        },
      ]}
      declared={{ guarantee: ['caucion'] }}
    />,
  );
  expect(
    await screen.findByText('Cumplís lo que pide: seguro de caución.'),
  ).toBeVisible();
});

// Each section title can explain itself in one sentence, on demand.
it('explains what each eligibility section means when asked', async () => {
  const user = userEvent.setup();
  render(
    <PropertyList
      listings={[
        { ...mockListings[0], rank: 1, eligibility: { state: 'eligible' } },
        {
          ...mockListings[1],
          rank: 2,
          eligibility: { state: 'conditionally_eligible' },
        },
      ]}
    />,
  );
  const meaning =
    'Cumplís lo publicado, pero una parte depende de la inmobiliaria o de tu ingreso exacto.';
  expect(screen.getByText(meaning)).not.toBeVisible();
  const [, conditional] = screen.getAllByText('¿Qué significa?');
  await user.click(conditional);
  expect(screen.getByText(meaning)).toBeVisible();
  expect(
    screen.getByText('Cumplís todo lo que el aviso publica como requisito.'),
  ).not.toBeVisible();
});

describe('PropertyList when nothing is left to show', () => {
  const refused = (n: number) =>
    Array.from({ length: n }, (_, i) => ({
      ...mockListings[0],
      id: 100 + i,
      url: `https://zonaprop.com.ar/refused-${i}`,
      address: `Refused ${i}`,
      rank: i + 1,
      eligibility: {
        state: 'ineligible' as const,
        conditions: [
          { reason: 'not_met', rule: { fact: 'guarantee', values: ['caucion'] } },
        ],
      },
    }));

  // Spec 009: the listings the searcher does not qualify for stay folded
  // until asked for, by click or keyboard, and focus lands on the first.
  it('opens and closes the listings the searcher does not qualify for', async () => {
    vi.stubGlobal('fetch', withFacts(() => Promise.reject(new Error('unreachable'))));
    const user = userEvent.setup();
    render(
      <PropertyList
        listings={[]}
        hiddenIneligible={2}
        ineligible={refused(2)}
        declared={{ guarantee: ['propietaria'] }}
      />,
    );
    const toggle = screen.getByRole('button', {
      name: 'Ver las 2 propiedades que no aceptan tu situación',
    });
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByText('Refused 0')).toBeNull();

    await user.click(toggle);
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByText('Refused 0')).toBeVisible();
    expect(screen.getByText('Refused 1')).toBeVisible();
    expect(screen.getAllByText('No calificás')).toHaveLength(2);
    expect(await screen.findAllByText('Pide seguro de caución')).toHaveLength(2);
    expect(document.activeElement).toBe(
      screen.getByText('Refused 0').closest('li'),
    );

    await user.click(toggle);
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByText('Refused 0')).toBeNull();

    toggle.focus();
    await user.keyboard('{Enter}');
    expect(screen.getByText('Refused 0')).toBeVisible();
    toggle.focus();
    await user.keyboard(' ');
    expect(screen.queryByText('Refused 0')).toBeNull();
  });

  // Found in review: the streamed results and the final reply carry the
  // same listings in two arrays; the section must not fold between them.
  it('stays open when the same listings arrive again, and folds for new ones', async () => {
    const user = userEvent.setup();
    const { rerender } = render(
      <PropertyList listings={[]} hiddenIneligible={2} ineligible={refused(2)} />,
    );
    await user.click(screen.getByRole('button', { name: /no aceptan tu situación/ }));
    rerender(
      <PropertyList listings={[]} hiddenIneligible={2} ineligible={refused(2)} />,
    );
    expect(screen.getByText('Refused 0')).toBeVisible();

    rerender(
      <PropertyList listings={[]} hiddenIneligible={1} ineligible={refused(3).slice(2)} />,
    );
    expect(screen.queryByText('Refused 2')).toBeNull();
  });

  it('names one listing in the singular and says when it shows only ten', () => {
    const { rerender } = render(
      <PropertyList listings={[]} hiddenIneligible={1} ineligible={refused(1)} />,
    );
    expect(
      screen.getByRole('button', {
        name: 'Ver la propiedad que no acepta tu situación',
      }),
    ).toBeVisible();

    rerender(
      <PropertyList listings={[]} hiddenIneligible={14} ineligible={refused(10)} />,
    );
    const toggle = screen.getByRole('button', {
      name: 'Ver las 14 propiedades que no aceptan tu situación',
    });
    toggle.click();
    return waitFor(() =>
      expect(
        screen.getByText('Te mostramos las 10 a las que les falta menos.'),
      ).toBeVisible(),
    );
  });

  it('offers nothing when the search shows results', () => {
    render(
      <PropertyList
        listings={[mockListings[0]]}
        hiddenIneligible={2}
        ineligible={refused(2)}
      />,
    );
    expect(screen.queryByRole('button', { name: /no acepta/ })).toBeNull();
  });
});
