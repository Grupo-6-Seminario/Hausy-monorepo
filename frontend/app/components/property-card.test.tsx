import { render, screen } from '@testing-library/react';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
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

  it('renders the property rank number', () => {
    render(<PropertyCard listing={{ ...sampleListing, rank: 1 }} />);

    expect(screen.getByText('#1')).toBeVisible();
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
