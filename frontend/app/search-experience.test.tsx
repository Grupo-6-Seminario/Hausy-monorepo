import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { SearchExperience } from './components/search-experience';

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

  it('sends the query when Enter is pressed', async () => {
    const user = userEvent.setup();
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ reply: 'Respuesta del agente.', requirements: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    vi.stubGlobal('fetch', fetchMock);
    render(<SearchExperience />);

    await user.type(screen.getByRole('textbox'), 'Dos ambientes con luz{Enter}');
    // The reply dialog is modal, so the textarea leaves the accessibility tree.
    await screen.findByRole('dialog', { name: 'Respuesta de Hausy' });

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/agent',
      expect.objectContaining({ body: expect.stringContaining('Dos ambientes con luz') }),
    );
    expect(document.querySelector<HTMLTextAreaElement>('#property-query')).toHaveValue(
      'Dos ambientes con luz',
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

  it('sends a nuanced query to the agent and shows its reply in a dialog', async () => {
    const user = userEvent.setup();
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          reply: 'Entendido. La luz natural es tu prioridad principal.',
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
      await screen.findByRole('dialog', { name: 'Respuesta de Hausy' }),
    ).toBeVisible();
    expect(
      screen.getByText('Entendido. La luz natural es tu prioridad principal.'),
    ).toBeVisible();
  });

  it('exposes the same search journey as a structured browser tool', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ reply: 'Respuesta del agente.', requirements: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
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

    await tool.execute({ query: 'Dos dormitorios con luz natural' });

    expect(await screen.findByText('Respuesta del agente.')).toBeVisible();
  });
});
