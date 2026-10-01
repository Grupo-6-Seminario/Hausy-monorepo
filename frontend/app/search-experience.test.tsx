import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

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
    {
      type: 'natural_light',
      value: 'high',
      provenance: 'stated',
      evidence: 'Muy luminoso',
    },
  ],
};
sampleListing.matched = sampleListing.attributes;

beforeEach(() => window.sessionStorage.clear());

afterEach(() => {
  Reflect.deleteProperty(document, 'modelContext');
  vi.unstubAllGlobals();
});

// localStorage is not available in this test environment; stub it the way the
// theme tests do.
function installStorage(initial: Record<string, string> = {}) {
  const values = new Map(Object.entries(initial));
  vi.stubGlobal('localStorage', {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => void values.set(key, value),
    removeItem: (key: string) => void values.delete(key),
  });
}

// The composer's own submit: the closing call to action shares its name.
// jsdom gives <search> no landmark role, so the form is reached via its field.
function composerSubmit() {
  const form = screen
    .getByRole('textbox', { name: /qué estás buscando/i })
    .closest('form') as HTMLFormElement;
  return within(form).getByRole('button', { name: /empezar búsqueda/i });
}

describe('SearchExperience', () => {
  it('uses the Hausy company name throughout the primary experience', () => {
    render(<SearchExperience />);
    const previousCompanyName = ['An', 'gus'].join('');

    expect(screen.getByRole('link', { name: 'Hausy, inicio' })).toBeVisible();
    expect(document.body).not.toHaveTextContent(previousCompanyName);
  });

  it('lights header controls where the pointer rests, without React state', () => {
    render(<SearchExperience />);
    const signIn = screen.getByRole('link', { name: 'Ingresar' });
    signIn.getBoundingClientRect = () => new DOMRect(100, 20, 84, 42);

    fireEvent.pointerOver(signIn, {
      pointerType: 'mouse',
      clientX: 130,
      clientY: 30,
    });
    expect(signIn.style.getPropertyValue('--glow-x')).toBe('30px');
    expect(signIn.style.getPropertyValue('--glow-y')).toBe('10px');

    fireEvent.pointerMove(signIn, {
      pointerType: 'mouse',
      clientX: 170,
      clientY: 52,
    });
    expect(signIn.style.getPropertyValue('--glow-x')).toBe('70px');
    expect(signIn.style.getPropertyValue('--glow-y')).toBe('32px');
  });

  it('makes the prompt the primary action with a green luminary canvas beneath it', () => {
    render(<SearchExperience />);

    const textbox = screen.getByRole('textbox', {
      name: /qué estás buscando/i,
    });
    expect(textbox).toBeVisible();
    expect(
      composerSubmit(),
    ).toBeVisible();
    expect(document.querySelector('[data-prompt-luminary]')).toBeInstanceOf(
      HTMLCanvasElement,
    );
    expect(document.querySelector('[data-prompt-stage]')).toHaveAttribute(
      'data-active',
      'false',
    );
  });

  it('presents the agent explanation as a readable recommendation brief', async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            reply: [
              '## Por qué las elegí',
              '1. **#1 Humboldt 1900**: prioriza la luz natural y mantiene el presupuesto.',
              '2. **#2 Thames 2200**: ofrece más silencio, pero queda un poco más lejos.',
              '',
              '## Qué falta confirmar',
              '- El aviso no publica orientación.',
            ].join('\n'),
            listings: [sampleListing],
            requirements: [],
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        ),
      ),
    );
    render(<SearchExperience />);

    await user.type(screen.getByRole('textbox'), 'Busco mucha luz{Enter}');

    const log = await screen.findByRole('log', {
      name: /conversación con Hausy/i,
    });
    const explanation = within(log).getByRole('region', {
      name: /explicación de Hausy/i,
    });
    expect(
      within(explanation).getByRole('heading', { name: 'Por qué las elegí' }),
    ).toBeVisible();
    expect(within(explanation).getAllByRole('listitem')).toHaveLength(3);
    expect(within(explanation).getByText('#1 Humboldt 1900')).toBeVisible();
    expect(within(explanation).queryByText(/\*\*/)).toBeNull();
    expect(screen.getByText('Destacada por Hausy')).toBeVisible();
  });

  it('keeps an empty query in place and explains what is missing', async () => {
    const user = userEvent.setup();
    render(<SearchExperience />);

    await user.click(composerSubmit());

    expect(screen.getByRole('alert')).toHaveTextContent(
      'Contanos al menos una necesidad o preferencia.',
    );
    expect(screen.getByRole('textbox')).toHaveFocus();
  });

  it('opens the workspace as one animated change before asking Hausy, and animates a follow-up as a turn', async () => {
    const user = userEvent.setup();
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        Response.json({ reply: 'Respuesta del agente.', listings: [] }),
      );
    vi.stubGlobal('fetch', fetchMock);
    // The browser captures the old view before it lets the page change.
    let captureOldView = () => {};
    const startViewTransition = vi.fn((update: () => void) => {
      const updated = new Promise<void>((resolve) => {
        captureOldView = () => {
          update();
          resolve();
        };
      });
      return {
        updateCallbackDone: updated,
        ready: updated,
        finished: new Promise<void>(() => {}),
      };
    });
    Object.defineProperty(document, 'startViewTransition', {
      configurable: true,
      value: startViewTransition,
    });
    try {
      render(<SearchExperience />);

      await user.type(screen.getByRole('textbox'), 'Dos ambientes{Enter}');

      expect(startViewTransition).toHaveBeenCalledOnce();
      expect(document.documentElement).toHaveAttribute(
        'data-morphing',
        'workspace',
      );
      expect(
        screen.getByRole('heading', {
          name: 'Primero, a qué podés acceder. Después, cuál te gusta.',
        }),
      ).toBeVisible();
      expect(fetchMock).not.toHaveBeenCalled();

      await act(async () => captureOldView());

      expect(
        screen.getByRole('heading', { name: 'Tu búsqueda' }),
      ).toBeVisible();
      expect(screen.getByText('Dos ambientes')).toBeVisible();
      await waitFor(() =>
        expect(fetchMock).toHaveBeenCalledWith('/api/agent', expect.anything()),
      );
      await screen.findByText('Respuesta del agente.');

      // A follow-up joins the conversation in place instead.
      await user.type(screen.getByRole('textbox'), 'Con balcón{Enter}');
      expect(document.documentElement).toHaveAttribute('data-morphing', 'turn');
      await act(async () => captureOldView());
      expect(screen.getByText('Con balcón')).toBeVisible();
    } finally {
      Reflect.deleteProperty(document, 'startViewTransition');
      document.documentElement.removeAttribute('data-morphing');
    }
  });

  it('opens with the qualification questions and prefills a signed-in profile', async () => {
    installStorage({ hausy_signed_in: '1' });
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(Response.json({ guarantee: ['caucion'] }));
    vi.stubGlobal('fetch', fetchMock);
    render(<SearchExperience />);

    expect(screen.getByLabelText('Garantía propietaria')).toBeVisible();
    await waitFor(() =>
      expect(screen.getByLabelText('Seguro de caución')).toBeChecked(),
    );
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/me/qualification',
      expect.anything(),
    );
  });

  it('shows the qualification in use and lets the user edit it', async () => {
    const user = userEvent.setup();
    const fetchMock = vi.fn().mockResolvedValue(
      Response.json({
        reply: 'Respuesta del agente.',
        listings: [
          { ...sampleListing, rank: 1, eligibility: { state: 'eligible' } },
        ],
      }),
    );
    vi.stubGlobal('fetch', fetchMock);
    render(<SearchExperience />);

    await user.click(screen.getByLabelText('Garantía propietaria'));
    await user.selectOptions(
      screen.getByLabelText('Ingresos mensuales'),
      '2000000-3000000',
    );
    await user.click(screen.getByRole('button', { name: 'Usar estos datos' }));
    await user.type(screen.getByRole('textbox'), 'Alquiler en Palermo{Enter}');

    expect(
      await screen.findByText(
        /^garantía propietaria · \$2\.000\.000 a \$3\.000\.000$/i,
      ),
    ).toBeVisible();
    expect(screen.queryByLabelText('Garantía propietaria')).toBeNull();
    await user.click(screen.getByRole('button', { name: 'Editar' }));
    // The panel opens beside the results with what was already declared.
    expect(screen.getByLabelText('Garantía propietaria')).toBeChecked();
    expect(screen.getByLabelText('Ingresos mensuales')).toHaveValue(
      '2000000-3000000',
    );
  });

  it('names the kind of each condition it is using', async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        Response.json({
          reply: 'Respuesta del agente.',
          listings: [sampleListing],
          requirements: [
            { type: 'outdoor_space', value: 'balcon' },
            { type: 'excluye_outdoor_space', value: 'balcon' },
          ],
        }),
      ),
    );
    render(<SearchExperience />);

    await user.type(screen.getByRole('textbox'), 'Con balcón{Enter}');

    const criteria = await screen.findByRole('list', {
      name: 'Criterios que estoy usando',
    });
    expect(
      within(criteria)
        .getAllByRole('listitem')
        .map((item) => item.textContent),
    ).toEqual(['outdoor space balcon', 'excluye outdoor space balcon']);
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
    await user.click(composerSubmit());

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/agent',
      expect.objectContaining({
        method: 'POST',
        body: expect.stringContaining(query),
      }),
    );

    expect(
      await screen.findByText(
        'Entendido. Encontré 1 propiedad con excelente luz natural.',
      ),
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

    expect(
      await screen.findByText('Primer turno: 1 propiedad encontrada.'),
    ).toBeVisible();
    expect(screen.getByText('Humboldt 1900')).toBeVisible();

    // Query box remains editable and in view for follow-up questions
    await user.clear(input);
    await user.type(input, '¿Tienen balcón?{Enter}');

    expect(
      await screen.findByText('Segundo turno: requisitos actualizados.'),
    ).toBeVisible();
    expect(screen.getByText('Thames 2200')).toBeVisible();
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it('keeps completed user and agent turns visible and clears the composer for a follow-up', async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            reply: 'Encontré una opción que prioriza luz y silencio.',
            listings: [sampleListing],
            requirements: [{ type: 'barrio', value: 'Palermo' }],
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        ),
      ),
    );
    render(<SearchExperience />);

    const composer = screen.getByRole('textbox', {
      name: /qué estás buscando/i,
    });
    await user.type(composer, 'Quiero vivir en Palermo con mucha luz{Enter}');

    expect(
      await screen.findByRole('log', { name: /conversación con Hausy/i }),
    ).toBeVisible();
    expect(
      screen.getByText('Quiero vivir en Palermo con mucha luz'),
    ).toBeVisible();
    expect(
      screen.getByText('Encontré una opción que prioriza luz y silencio.'),
    ).toBeVisible();
    expect(composer).toHaveValue('');
    expect(composer).toHaveFocus();
  });

  it('keeps the current shortlist visible while Hausy handles a follow-up', async () => {
    const user = userEvent.setup();
    let finishFollowUp: ((response: Response) => void) | undefined;
    const followUpResponse = new Promise<Response>((resolve) => {
      finishFollowUp = resolve;
    });
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            reply: 'Primera selección.',
            listings: [sampleListing],
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        ),
      )
      .mockReturnValueOnce(followUpResponse);
    vi.stubGlobal('fetch', fetchMock);
    render(<SearchExperience />);

    const composer = screen.getByRole('textbox');
    await user.type(composer, 'Busco algo luminoso{Enter}');
    expect(await screen.findByText('Primera selección.')).toBeVisible();

    await user.type(composer, '¿Y si priorizamos silencio?{Enter}');

    expect(await screen.findByRole('status')).toHaveTextContent(
      'Consultando el inventario y comparando tus prioridades',
    );
    expect(document.querySelector('[data-prompt-stage]')).toHaveAttribute(
      'data-active',
      'true',
    );
    expect(screen.getByText('Humboldt 1900')).toBeVisible();

    finishFollowUp?.(
      new Response(
        JSON.stringify({
          reply: 'Selección actualizada.',
          listings: [sampleListing],
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    );
    expect(await screen.findByText('Selección actualizada.')).toBeVisible();
  });

  it('lets the user stop a slow request and restores the message for editing', async () => {
    const user = userEvent.setup();
    const fetchMock = vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      return new Promise<Response>((_resolve, reject) => {
        init?.signal?.addEventListener('abort', () =>
          reject(new DOMException('Aborted', 'AbortError')),
        );
      });
    });
    vi.stubGlobal('fetch', fetchMock);
    render(<SearchExperience />);

    const composer = screen.getByRole('textbox');
    await user.type(composer, 'Dos dormitorios y poco ruido{Enter}');
    await user.click(await screen.findByRole('button', { name: /detener/i }));

    expect(fetchMock.mock.calls[0][1]?.signal).toHaveProperty('aborted', true);
    expect(composer).toHaveValue('Dos dormitorios y poco ruido');
    expect(composer).toHaveFocus();
    expect(screen.queryByRole('status')).toBeNull();
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

    const result = await tool.execute({
      query: 'Dos dormitorios con luz natural',
    });

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
