import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Listing } from '@/lib/types';
import { SearchExperience } from './components/search-experience';

const sampleListing: Listing = {
  id: 101,
  source: 'zonaprop',
  url: 'https://www.zonaprop.com.ar/101',
  neighborhood: 'palermo',
  address: 'Humboldt 1900',
  description: 'Muy luminoso',
  price: { amount: 900, currency: 'USD' },
  expenses: { amount: 110000, currency: 'ARS' },
  rooms: 2,
  bedrooms: 1,
  bathrooms: 1,
  total_area_m2: 50,
  attributes: [
    { type: 'natural_light', value: 'high', provenance: 'stated', evidence: 'Muy luminoso' },
  ],
};

afterEach(() => {
  Reflect.deleteProperty(document, 'modelContext');
  vi.unstubAllGlobals();
});

describe('SearchExperience', () => {
  it('uses the Hausy company name throughout the primary experience', () => {
    render(<SearchExperience />);
    const previousCompanyName = ['An', 'gus'].join('');

    expect(screen.getByRole('link', { name: 'Hausy, inicio' })).toBeVisible();
    expect(document.body).not.toHaveTextContent(previousCompanyName);
  });

  it('makes a natural-language property query the primary action', () => {
    render(<SearchExperience />);

    const textbox = screen.getByRole('textbox', {
      name: /describí cómo querés vivir/i,
    });
    const ledBorder = document.querySelector('[data-prompt-led-border]');

    expect(textbox).toBeVisible();
    expect(screen.getByRole('button', { name: /buscar hogares/i })).toBeVisible();
    expect(ledBorder).toBeInstanceOf(HTMLCanvasElement);
    expect(ledBorder?.nextElementSibling).toBe(textbox);
  });

  it('keeps an empty query in place and explains what is missing', async () => {
    const user = userEvent.setup();
    render(<SearchExperience />);

    await user.click(screen.getByRole('button', { name: /buscar hogares/i }));

    expect(screen.getByRole('alert')).toHaveTextContent(
      'Contanos al menos una necesidad o preferencia.',
    );
    expect(screen.getByRole('textbox')).toHaveFocus();
  });

  it('sends the query when Enter is pressed and displays results inline', async () => {
    const user = userEvent.setup();
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          reply: 'Respuesta del agente.',
          listings: [sampleListing],
          requirements: [],
        }),
        {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        },
      ),
    );
    vi.stubGlobal('fetch', fetchMock);
    render(<SearchExperience />);

    await user.type(screen.getByRole('textbox'), 'Dos ambientes con luz{Enter}');

    expect(await screen.findByText('Respuesta de Hausy')).toBeVisible();
    expect(screen.getByText('Respuesta del agente.')).toBeVisible();
    expect(screen.getByText('Humboldt 1900')).toBeVisible();

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/agent',
      expect.objectContaining({ body: expect.stringContaining('Dos ambientes con luz') }),
    );
  });

  it('starts a new line on Shift+Enter without sending', async () => {
    const user = userEvent.setup();
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    render(<SearchExperience />);

    await user.type(
      screen.getByRole('textbox'),
      'Mucha luz{Shift>}{Enter}{/Shift}y poco ruido',
    );

    expect(screen.getByRole('textbox')).toHaveValue('Mucha luz\ny poco ruido');
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('sends a nuanced query and renders property listings in box card fashion inline', async () => {
    const user = userEvent.setup();
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          reply: 'Entendido. Encontré 1 propiedad con excelente luz natural.',
          listings: [sampleListing],
          requirements: [],
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    );
    vi.stubGlobal('fetch', fetchMock);
    render(<SearchExperience />);
    const query =
      'Busco dos dormitorios en Palermo, hasta USD 1.000. Priorizo luz natural y poco ruido por encima del balcón.';

    await user.type(screen.getByRole('textbox'), query);
    await user.click(screen.getByRole('button', { name: /buscar hogares/i }));

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/agent',
      expect.objectContaining({
        method: 'POST',
        body: expect.stringContaining(query),
      }),
    );

    expect(
      await screen.findByText('Entendido. Encontré 1 propiedad con excelente luz natural.'),
    ).toBeVisible();
    expect(screen.getByText('Humboldt 1900')).toBeVisible();
    expect(screen.getByText(/USD 900/i)).toBeVisible();
    expect(screen.getByText(/luz natural: alta/i)).toBeVisible();
    // Modal dialog is NOT rendered
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('allows multi-turn interaction without blocking modal dialogs', async () => {
    const user = userEvent.setup();
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            reply: 'Primer turno: 1 propiedad encontrada.',
            listings: [sampleListing],
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            reply: 'Segundo turno: requisitos actualizados.',
            listings: [
              {
                ...sampleListing,
                id: 102,
                address: 'Thames 2200',
              },
            ],
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        ),
      );
    vi.stubGlobal('fetch', fetchMock);
    render(<SearchExperience />);

    const input = screen.getByRole('textbox');
    await user.type(input, 'Busco en Palermo{Enter}');

    expect(await screen.findByText('Primer turno: 1 propiedad encontrada.')).toBeVisible();
    expect(screen.getByText('Humboldt 1900')).toBeVisible();

    // Query box remains editable and in view for follow-up questions
    await user.clear(input);
    await user.type(input, '¿Tienen balcón?{Enter}');

    expect(await screen.findByText('Segundo turno: requisitos actualizados.')).toBeVisible();
    expect(screen.getByText('Thames 2200')).toBeVisible();
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it('exposes the same search journey as a structured browser tool returning listings', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            reply: 'Respuesta del agente.',
            listings: [sampleListing],
            requirements: [],
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      ),
    );
    const registerTool = vi.fn();
    Object.defineProperty(document, 'modelContext', {
      configurable: true,
      value: { registerTool },
    });
    render(<SearchExperience />);

    expect(registerTool).toHaveBeenCalledOnce();
    const tool = registerTool.mock.calls[0][0];
    expect(tool.name).toBe('search_properties');

    const result = await tool.execute({ query: 'Dos dormitorios con luz natural' });

    expect(await screen.findByText('Respuesta del agente.')).toBeVisible();
    expect(screen.getByText('Humboldt 1900')).toBeVisible();
    expect(result).toMatchObject({
      status: 'complete',
      query: 'Dos dormitorios con luz natural',
      reply: 'Respuesta del agente.',
      listings: [sampleListing],
    });
  });
});
