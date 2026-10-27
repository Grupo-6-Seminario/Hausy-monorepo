import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { factsFixture } from '@/lib/eligibility-facts.fixture';

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
    inquiry_count: 0,
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
    requirements: [],
  },
];

// GET /api/agency/inquiries as the server words one inquiry.
const inquiry = {
  id: '5f0c1d2e-8a4b-4c6d-9e7f-0a1b2c3d4e5f',
  listing_id: '15',
  property: 'Soler al 4200',
  agency: 'Marta',
  name: 'Ana',
  email: 'ana@example.com',
  phone: '+54 9 11 5555-0000',
  message: '¿Se puede visitar el sábado?',
  searching: ['2 ambientes', 'en Palermo'],
  eligibility: 'unknown',
  requirements: [
    {
      fact: 'guarantee',
      label: 'Garantía',
      requirement: 'Acepta garantía propietaria o seguro de caución',
      hardness: 'hard',
      declared: ['Garantía propietaria'],
      outcome: 'met',
    },
    {
      fact: 'income_band',
      label: 'Ingresos mensuales',
      requirement: 'Ingresos de 3 veces el alquiler',
      hardness: 'discretionary',
      declared: ['$2.000.000 a $3.000.000'],
      outcome: 'discretionary',
    },
    {
      fact: 'pets',
      label: 'Mascotas',
      requirement: 'No admite mascotas',
      hardness: 'hard',
      declared: [],
      outcome: 'missing',
    },
  ],
  created_at: '2026-10-26T12:00:00Z',
};

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function stubDashboard(
  catalog: typeof properties,
  inquiries: (typeof inquiry)[] = [],
) {
  const fetchMock = vi.fn((input: string, init?: RequestInit) => {
    if (input === '/api/auth/me') return Promise.resolve(json({ user: marta }));
    if (input === '/api/eligibility/facts') {
      return Promise.resolve(json({ facts: factsFixture }));
    }
    if (input === '/api/agency/inquiries') {
      return Promise.resolve(json({ inquiries }));
    }
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
              inquiry_count: 0,
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
  it('adds a property with editable facts only', async () => {
    const user = userEvent.setup();
    const fetchMock = stubDashboard([]);
    render(<AgencyDashboard />);
    await screen.findByRole('heading', { name: 'Tu catálogo' });

    await user.click(screen.getByRole('button', { name: 'Agregar propiedad' }));
    // Long values are pasted: typing them key by key re-renders the form on
    // every key and pushed this test past the 5 s timeout under a full run.
    await user.click(screen.getByLabelText('URL de la publicación'));
    await user.paste(
      'https://www.zonaprop.com.ar/propiedades/palermo-303.html',
    );
    await user.type(screen.getByLabelText('Dirección'), 'Soler al 4200');
    await user.type(screen.getByLabelText('Barrio'), 'palermo');
    await user.type(screen.getByLabelText('Precio'), '950');
    await user.selectOptions(screen.getByLabelText('Moneda'), 'USD');
    await user.click(screen.getByLabelText('Descripción'));
    await user.paste('Departamento con balcón al frente.');
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

  it('archives a property while retaining its received inquiries', async () => {
    const user = userEvent.setup();
    const fetchMock = stubDashboard(
      [{ ...properties[0], inquiry_count: 1 }],
      [{ ...inquiry, listing_id: '14', property: properties[0].address }],
    );
    render(<AgencyDashboard />);
    await screen.findByRole('button', { name: 'Quitar' });

    await user.click(screen.getByRole('button', { name: 'Quitar' }));
    expect(
      screen.getByRole('alertdialog', { name: 'Quitar Humboldt al 1900' }),
    ).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Confirmar quitar' }));

    expect(await screen.findByText('Tu catálogo está vacío')).toBeVisible();
    expect(screen.getByText('1 consulta')).toBeVisible();
    expect(
      within(screen.getByRole('region', { name: 'Consultas' })).getByText(
        'Ana',
      ),
    ).toBeVisible();
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/agency/catalog/14',
      expect.objectContaining({ method: 'DELETE' }),
    );
  });

  it('declares the requirements to rent with the property', async () => {
    const user = userEvent.setup();
    const fetchMock = stubDashboard([]);
    render(<AgencyDashboard />);
    await screen.findByRole('heading', { name: 'Tu catálogo' });

    await user.click(screen.getByRole('button', { name: 'Agregar propiedad' }));
    await user.click(screen.getByLabelText('URL de la publicación'));
    await user.paste(
      'https://www.zonaprop.com.ar/propiedades/palermo-303.html',
    );
    await user.type(screen.getByLabelText('Barrio'), 'palermo');
    await user.click(screen.getByLabelText('Descripción'));
    await user.paste('Departamento con balcón al frente.');

    const guarantees = screen.getByRole('group', {
      name: 'Garantías aceptadas',
    });
    await user.click(
      await within(guarantees).findByLabelText('Garantía propietaria'),
    );
    await user.click(within(guarantees).getByLabelText('Seguro de caución'));
    const income = screen.getByRole('group', { name: 'Ingresos' });
    await user.type(within(income).getByLabelText('Veces el alquiler'), '2.5');
    await user.click(within(income).getByLabelText('Sumando expensas'));
    await user.selectOptions(
      within(income).getByLabelText('Exigencia'),
      'A criterio de la inmobiliaria',
    );
    // A strictness alone declares nothing: the server would read an empty
    // pets requirement as "no pets".
    await user.selectOptions(
      within(screen.getByRole('group', { name: 'Mascotas' })).getByLabelText(
        'Exigencia',
      ),
      'A criterio de la inmobiliaria',
    );
    await user.click(screen.getByRole('button', { name: 'Guardar propiedad' }));

    expect(await screen.findByText('Propiedad en palermo')).toBeVisible();
    const postCall = fetchMock.mock.calls.find(
      ([input, init]) =>
        input === '/api/agency/catalog' && init?.method === 'POST',
    );
    const { requirements } = JSON.parse(postCall?.[1]?.body as string);
    expect(requirements).toHaveLength(2);
    expect(requirements).toEqual(
      expect.arrayContaining([
        {
          fact: 'guarantee',
          operator: 'one_of',
          values: ['propietaria', 'caucion'],
          hardness: 'hard',
        },
        {
          fact: 'income_band',
          operator: 'income_multiple_with_expenses',
          values: ['2.5'],
          hardness: 'discretionary',
        },
      ]),
    );
  });

  it('lists each inquiry with what the person declared and how to answer', async () => {
    stubDashboard([{ ...properties[0], inquiry_count: 1 }], [inquiry]);
    render(<AgencyDashboard />);
    const inbox = await screen.findByRole('region', { name: 'Consultas' });

    expect(screen.getByText('1 consulta')).toBeVisible();
    expect(
      within(inbox).getByRole('heading', { name: 'Soler al 4200' }),
    ).toBeVisible();
    expect(within(inbox).getByText('Ana')).toBeVisible();
    expect(within(inbox).getByText(/26 oct 2026/)).toBeVisible();
    expect(within(inbox).getByText('Faltan datos')).toBeVisible();
    const rows = within(
      within(inbox).getByRole('list', { name: 'Requisitos de la propiedad' }),
    ).getAllByRole('listitem');
    expect(
      rows.map((row) => [...row.children].map((cell) => cell.textContent)),
    ).toEqual([
      [
        'Acepta garantía propietaria o seguro de caución',
        'Declaró: Garantía propietaria',
        'Cumple',
      ],
      [
        'Ingresos de 3 veces el alquiler',
        'Declaró: $2.000.000 a $3.000.000',
        'A tu criterio',
      ],
      ['No admite mascotas', 'No lo declaró', 'Falta el dato'],
    ]);
    expect(
      within(inbox).getByText('Busca: 2 ambientes · en Palermo'),
    ).toBeVisible();
    expect(
      within(inbox).getByText('¿Se puede visitar el sábado?'),
    ).toBeVisible();
    expect(
      within(inbox).getByRole('link', { name: 'ana@example.com' }),
    ).toHaveAttribute('href', 'mailto:ana@example.com');
    expect(
      within(inbox).getByRole('link', { name: 'WhatsApp +54 9 11 5555-0000' }),
    ).toHaveAttribute('href', 'https://wa.me/5491155550000');
  });

  it('says where inquiries will appear before the first one arrives', async () => {
    stubDashboard([properties[0]]);
    render(<AgencyDashboard />);
    const inbox = await screen.findByRole('region', { name: 'Consultas' });

    expect(
      within(inbox).getByText('Todavía no llegaron consultas'),
    ).toBeVisible();
    expect(
      within(inbox).getByText(
        'Aparecen acá cuando alguien te escribe desde Hausy.',
      ),
    ).toBeVisible();
  });
});
