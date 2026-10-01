import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import type { Listing } from '@/lib/types';
import { PropertyList } from './property-list';

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
        relaxations={[{ fact: 'guarantee', value: 'caucion', count: 2 }]}
      />,
    );

    const headings = screen
      .getAllByRole('heading', { level: 3 })
      .map((h) => h.textContent);
    expect(headings).toEqual([
      'Calificás',
      'Depende de la inmobiliaria',
      'A confirmar',
    ]);
    expect(screen.getByText(/ver cuáles permite la propietaria/)).toBeVisible();
    expect(screen.getByText('#3')).toBeVisible();
    expect(
      screen.getByText('Ordenadas por si podés alquilarlas'),
    ).toBeVisible();
    expect(
      screen.getByText(/Si conseguís seguro de caución, vuelven 2 propiedades/),
    ).toBeVisible();
  });
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
