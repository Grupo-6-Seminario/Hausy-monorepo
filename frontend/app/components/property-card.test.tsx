import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import type { Listing } from '@/lib/types';
import { PropertyCard } from './property-card';

const sampleListing: Listing = {
  id: 101,
  source: 'zonaprop',
  url: 'https://www.zonaprop.com.ar/propiedades/departamento-palermo-101.html',
  neighborhood: 'palermo',
  agency: 'LOUNGE PROPIEDADES S.A.',
  address: 'Humboldt al 1900, Palermo',
  description: 'Excelente semipiso al contrafrente con mucha luz natural y vista abierta.',
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

describe('PropertyCard', () => {
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
});
