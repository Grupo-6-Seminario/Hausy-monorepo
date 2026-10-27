import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Inquiry } from '@/lib/agency';
import { searchingPhrases } from '@/lib/inquiry';
import { requirementLabel } from '@/lib/listing-labels';
import type { Listing } from '@/lib/types';

import { InquiryDialog } from './inquiry-dialog';
import { PropertyCard } from './property-card';

const preview: Inquiry = {
  listing_id: '12',
  preview_revision: 'revision-1',
  property: 'Gorriti 4800',
  agency: 'Inmobiliaria Demo',
  name: '',
  searching: ['2 ambientes', 'Palermo'],
  eligibility: 'conditionally_eligible',
  requirements: [
    {
      fact: 'guarantee',
      label: 'Garantía',
      requirement: 'Acepta garantía propietaria o seguro de caución',
      hardness: 'hard',
      declared: ['Seguro de caución'],
      outcome: 'met',
    },
    {
      fact: 'pets',
      label: 'Mascotas',
      requirement: 'Admite gato',
      hardness: 'discretionary',
      declared: [],
      outcome: 'missing',
    },
  ],
};

function respondWith(body: object, status = 200) {
  return Promise.resolve(Response.json(body, { status }));
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('InquiryDialog', () => {
  it('shows what the agency would receive, then sends it with the same input', async () => {
    const fetchMock = vi
      .fn()
      .mockImplementationOnce(() => respondWith(preview))
      .mockImplementationOnce(() =>
        respondWith({ ...preview, name: 'Ana' }, 201),
      );
    vi.stubGlobal('fetch', fetchMock);
    const user = userEvent.setup();

    render(
      <InquiryDialog
        listingId="12"
        title="Gorriti 4800"
        agency="Inmobiliaria Demo"
        declared={{ guarantee: ['caucion'] }}
        searching={['2 ambientes', 'Palermo']}
        onClose={() => {}}
      />,
    );

    expect(
      await screen.findByText(
        'Acepta garantía propietaria o seguro de caución',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText('Declaraste: Seguro de caución'),
    ).toBeInTheDocument();
    expect(screen.getByText('Cumplís')).toBeInTheDocument();
    expect(screen.getByText('No lo declaraste')).toBeInTheDocument();
    expect(screen.getByText('Falta este dato')).toBeInTheDocument();
    expect(
      screen.getByText('Lo que buscás: 2 ambientes · Palermo'),
    ).toBeInTheDocument();

    await user.type(screen.getByLabelText('Tu nombre'), 'Ana');
    await user.type(screen.getByLabelText('Email'), 'ana@example.com');
    await user.click(screen.getByRole('button', { name: 'Enviar consulta' }));

    expect(
      await screen.findByText(/tu consulta le llegó a Inmobiliaria Demo/),
    ).toBeInTheDocument();
    const [previewCall, sendCall] = fetchMock.mock.calls;
    expect(previewCall[0]).toBe('/api/listings/12/inquiries/preview');
    expect(sendCall[0]).toBe('/api/listings/12/inquiries');
    const sent = JSON.parse(sendCall[1].body);
    expect(sent).toMatchObject({
      name: 'Ana',
      email: 'ana@example.com',
      preview_revision: 'revision-1',
      qualification: { guarantee: ['caucion'] },
      searching: ['2 ambientes', 'Palermo'],
    });
    expect(sent.inquiry_id).toMatch(/^[0-9a-f-]{36}$/);
  });

  it('refreshes changed requirements and asks for consent again without losing contact details', async () => {
    const updated = {
      ...preview,
      preview_revision: 'revision-2',
      requirements: [
        {
          ...preview.requirements[0],
          requirement: 'Acepta únicamente seguro de caución',
        },
      ],
    };
    const fetchMock = vi
      .fn()
      .mockImplementationOnce(() => respondWith(preview))
      .mockImplementationOnce(() =>
        respondWith(
          {
            code: 'preview_changed',
            error: 'La propiedad cambió. Revisá la consulta antes de enviarla.',
          },
          409,
        ),
      )
      .mockImplementationOnce(() => respondWith(updated))
      .mockImplementationOnce(() =>
        respondWith({ ...updated, name: 'Ana' }, 201),
      );
    vi.stubGlobal('fetch', fetchMock);
    const user = userEvent.setup();
    render(
      <InquiryDialog
        listingId="12"
        title="Gorriti 4800"
        declared={{ guarantee: ['caucion'] }}
        searching={[]}
        onClose={() => {}}
      />,
    );
    await screen.findByText('Admite gato');
    await user.type(screen.getByLabelText('Tu nombre'), 'Ana');
    await user.type(screen.getByLabelText('Email'), 'ana@example.com');
    await user.click(screen.getByRole('button', { name: 'Enviar consulta' }));
    expect(
      await screen.findByText('Acepta únicamente seguro de caución'),
    ).toBeVisible();
    expect(screen.getByRole('alert')).toHaveTextContent('La propiedad cambió');
    expect(screen.getByLabelText('Tu nombre')).toHaveValue('Ana');
    expect(fetchMock).toHaveBeenCalledTimes(3);
    await user.click(screen.getByRole('button', { name: 'Enviar consulta' }));
    await screen.findByText(/tu consulta le llegó/);
    const firstSend = JSON.parse(fetchMock.mock.calls[1][1].body);
    const secondSend = JSON.parse(fetchMock.mock.calls[3][1].body);
    expect(secondSend.preview_revision).toBe('revision-2');
    expect(secondSend.inquiry_id).toBe(firstSend.inquiry_id);
  });

  it('shows the stored contact when a retry returns an earlier receipt', async () => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockImplementationOnce(() => respondWith(preview))
        .mockImplementationOnce(() =>
          Promise.reject(new Error('Se cortó la conexión.')),
        )
        .mockImplementationOnce(() =>
          respondWith({
            ...preview,
            name: 'Ana',
            email: 'original@example.com',
          }),
        ),
    );
    const user = userEvent.setup();
    render(
      <InquiryDialog
        listingId="12"
        title="Gorriti 4800"
        declared={{}}
        searching={[]}
        onClose={() => {}}
      />,
    );
    await screen.findByText('Admite gato');
    await user.type(screen.getByLabelText('Tu nombre'), 'Ana');
    await user.type(screen.getByLabelText('Email'), 'original@example.com');
    await user.click(screen.getByRole('button', { name: 'Enviar consulta' }));
    await screen.findByRole('alert');
    await user.clear(screen.getByLabelText('Email'));
    await user.type(screen.getByLabelText('Email'), 'edited@example.com');
    await user.click(screen.getByRole('button', { name: 'Enviar consulta' }));
    const receipt = await screen.findByRole('status');
    expect(receipt).toHaveTextContent('original@example.com');
    expect(receipt).not.toHaveTextContent('edited@example.com');
  });

  it('keeps the form and says why when the server refuses the send', async () => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockImplementationOnce(() => respondWith(preview))
        .mockImplementationOnce(() =>
          respondWith(
            {
              error:
                'Dejá un email o un WhatsApp para que la inmobiliaria te responda.',
            },
            400,
          ),
        ),
    );
    const user = userEvent.setup();
    render(
      <InquiryDialog
        listingId="12"
        title="Gorriti 4800"
        declared={{}}
        searching={[]}
        onClose={() => {}}
      />,
    );

    await screen.findByText('Admite gato');
    await user.type(screen.getByLabelText('Tu nombre'), 'Ana');
    await user.click(screen.getByRole('button', { name: 'Enviar consulta' }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Dejá un email o un WhatsApp para que la inmobiliaria te responda.',
    );
    expect(screen.getByLabelText('Tu nombre')).toHaveValue('Ana');
  });
});

describe('searchingPhrases', () => {
  it('never tells the agency the budget', () => {
    expect(
      searchingPhrases(
        [
          { type: 'neighborhood', value: 'palermo' },
          { type: 'max_price', value: '900000' },
          { type: 'max_expenses', value: '100000' },
        ],
        requirementLabel,
      ),
    ).toEqual(['Palermo']);
  });
});

describe('PropertyCard inquiry action', () => {
  const managed: Listing = {
    id: 12,
    source: 'agency',
    url: 'https://example.com/inmobiliaria-demo/gorriti-4800',
    neighborhood: 'palermo',
    agency: 'Inmobiliaria Demo',
    address: 'Gorriti 4800',
    description: 'Dos ambientes.',
    price: { amount: 850000, currency: 'ARS' },
    expenses: { amount: 110000, currency: 'ARS' },
    managed: true,
    eligibility: { state: 'eligible' },
  };

  it('offers an inquiry on a listing an agency manages in Hausy', () => {
    render(<PropertyCard listing={managed} />);
    expect(
      screen.getByRole('button', { name: 'Enviar consulta' }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('link', { name: /^Contactar/ }),
    ).toBeInTheDocument();
  });

  it('offers none on a scraped listing or to someone a strict requirement rules out', () => {
    const { rerender } = render(
      <PropertyCard listing={{ ...managed, managed: false }} />,
    );
    expect(
      screen.queryByRole('button', { name: 'Enviar consulta' }),
    ).toBeNull();
    rerender(
      <PropertyCard
        listing={{ ...managed, eligibility: { state: 'ineligible' } }}
      />,
    );
    expect(
      screen.queryByRole('button', { name: 'Enviar consulta' }),
    ).toBeNull();
  });

  it('opens the dialog for that listing', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() => respondWith(preview)),
    );
    const user = userEvent.setup();
    render(<PropertyCard listing={managed} />);

    await user.click(screen.getByRole('button', { name: 'Enviar consulta' }));

    await waitFor(() =>
      expect(
        screen.getByRole('dialog', { name: 'Gorriti 4800' }),
      ).toBeInTheDocument(),
    );
  });
});
