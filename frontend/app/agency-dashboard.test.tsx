import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AgencyDashboard } from './components/agency-dashboard';

const marta = {
  id: '7',
  email: 'marta@inmobiliaria.com',
  name: 'Marta',
  role: 'realtor',
};

const properties = [
  {
    id: '14',
    agency: 'Marta',
    source: 'agency',
    contact_count: 7,
    url: 'https://www.zonaprop.com.ar/propiedades/palermo-101.html',
    neighborhood: 'palermo',
    address: 'Humboldt al 1900',
    description: 'Semipiso luminoso al contrafrente.',
    operation: 'alquiler',
    price: { amount: 850, currency: 'USD' },
    expenses: { amount: 120000, currency: 'ARS' },
    rooms: 3,
    bedrooms: 2,
    bathrooms: 1,
    total_area_m2: 65,
  },
  {
    id: '15',
    agency: 'Marta',
    source: 'agency',
    contact_count: 6,
    url: 'https://www.zonaprop.com.ar/propiedades/belgrano-202.html',
    neighborhood: 'belgrano',
    address: 'Mendoza al 2400',
    description: 'Departamento de dos ambientes.',
    operation: 'alquiler',
    price: { amount: 720000, currency: 'ARS' },
    expenses: { amount: null, currency: '' },
    rooms: 2,
    bedrooms: 1,
    bathrooms: 1,
    total_area_m2: 48,
  },
];

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function stubDashboard(catalog = properties) {
  const fetchMock = vi.fn((input: string, init?: RequestInit) => {
    if (input === '/api/auth/me') return Promise.resolve(json({ user: marta }));
    if (input === '/api/agency/catalog') {
      if (init?.method === 'POST') {
        const inputProperty = JSON.parse(init.body as string);
        return Promise.resolve(
          json(
            {
              id: '16',
              agency: 'Marta',
              source: 'agency',
              contact_count: 0,
              ...inputProperty,
            },
            201,
          ),
        );
      }
      return Promise.resolve(json({ properties: catalog }));
    }
    if (input.startsWith('/api/agency/catalog/') && init?.method === 'PATCH') {
      const inputProperty = JSON.parse(init.body as string);
      return Promise.resolve(
        json({ ...catalog[0], ...inputProperty, contact_count: 7 }),
      );
    }
    if (input.startsWith('/api/agency/catalog/') && init?.method === 'DELETE') {
      return Promise.resolve(new Response(null, { status: 204 }));
    }
    return Promise.resolve(json({ error: 'unexpected request' }, 500));
  });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('AgencyDashboard', () => {
  it('shows an authenticated realtor their catalog and contact totals', async () => {
    stubDashboard();
    render(<AgencyDashboard />);

    expect(
      await screen.findByRole('heading', { name: 'Tu catálogo' }),
    ).toBeVisible();
    expect(screen.getByText('2 propiedades activas')).toBeVisible();
    expect(screen.getByText('13 contactos iniciados')).toBeVisible();
    expect(screen.getByText('Humboldt al 1900')).toBeVisible();
    expect(screen.getByText('Mendoza al 2400')).toBeVisible();
    expect(
      screen.getByRole('button', { name: 'Agregar propiedad' }),
    ).toBeVisible();
  });

  it('adds a property with editable facts only', async () => {
    const user = userEvent.setup();
    const fetchMock = stubDashboard([]);
    render(<AgencyDashboard />);
    await screen.findByRole('heading', { name: 'Tu catálogo' });

    await user.click(screen.getByRole('button', { name: 'Agregar propiedad' }));
    await user.type(
      screen.getByLabelText('URL de la publicación'),
      'https://www.zonaprop.com.ar/propiedades/palermo-303.html',
    );
    await user.type(screen.getByLabelText('Dirección'), 'Soler al 4200');
    await user.type(screen.getByLabelText('Barrio'), 'palermo');
    await user.type(screen.getByLabelText('Precio'), '950');
    await user.selectOptions(screen.getByLabelText('Moneda'), 'USD');
    await user.type(
      screen.getByLabelText('Descripción'),
      'Departamento con balcón al frente.',
    );
    await user.click(screen.getByRole('button', { name: 'Guardar propiedad' }));

    expect(await screen.findByText('Soler al 4200')).toBeVisible();
    const postCall = fetchMock.mock.calls.find(
      ([input, init]) =>
        input === '/api/agency/catalog' && init?.method === 'POST',
    );
    expect(JSON.parse(postCall?.[1]?.body as string)).toEqual(
      expect.objectContaining({
        url: 'https://www.zonaprop.com.ar/propiedades/palermo-303.html',
        neighborhood: 'palermo',
        address: 'Soler al 4200',
        operation: 'alquiler',
        price: { amount: 950, currency: 'USD' },
      }),
    );
    expect(JSON.parse(postCall?.[1]?.body as string)).not.toHaveProperty(
      'contact_count',
    );
    expect(JSON.parse(postCall?.[1]?.body as string)).not.toHaveProperty(
      'owner_id',
    );
  });

  it('edits property facts while keeping the observed contact total', async () => {
    const user = userEvent.setup();
    const fetchMock = stubDashboard([properties[0]]);
    render(<AgencyDashboard />);
    await screen.findByText('Humboldt al 1900');

    await user.click(screen.getByRole('button', { name: 'Editar' }));
    const address = screen.getByLabelText('Dirección');
    expect(address).toHaveValue('Humboldt al 1900');
    await user.clear(address);
    await user.type(address, 'Humboldt al 2000');
    await user.click(screen.getByRole('button', { name: 'Guardar cambios' }));

    expect(await screen.findByText('Humboldt al 2000')).toBeVisible();
    expect(screen.getByText('7')).toBeVisible();
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/agency/catalog/14',
      expect.objectContaining({ method: 'PATCH' }),
    );
  });

  it('confirms catalog removal before archiving the property', async () => {
    const user = userEvent.setup();
    const fetchMock = stubDashboard([properties[0]]);
    render(<AgencyDashboard />);
    await screen.findByText('Humboldt al 1900');

    await user.click(screen.getByRole('button', { name: 'Quitar' }));
    expect(
      screen.getByRole('alertdialog', { name: 'Quitar Humboldt al 1900' }),
    ).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Confirmar quitar' }));

    expect(await screen.findByText('Tu catálogo está vacío')).toBeVisible();
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/agency/catalog/14',
      expect.objectContaining({ method: 'DELETE' }),
    );
  });
});
