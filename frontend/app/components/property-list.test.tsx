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
    attributes: [{ type: 'natural_light', value: 'high', provenance: 'stated' }],
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
    attributes: [{ type: 'noise_level', value: 'quiet', provenance: 'inferred' }],
  },
];

describe('PropertyList', () => {
  it('renders a list of property cards with count', () => {
    render(<PropertyList listings={mockListings} />);

    expect(screen.getByRole('region', { name: /propiedades encontradas/i })).toBeVisible();
    expect(screen.getByText(/2 propiedades seleccionadas/i)).toBeVisible();
    expect(screen.getByText(/Humboldt 1900/i)).toBeVisible();
    expect(screen.getByText(/Cabildo 2000/i)).toBeVisible();
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
