import { act, fireEvent, waitFor } from '@testing-library/react';
import { hydrateRoot } from 'react-dom/client';
import { renderToString } from 'react-dom/server';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { SearchExperience } from './search-experience';

let fetchMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
	window.sessionStorage.clear();
  fetchMock = vi.fn(
    async () => new Response(JSON.stringify({ reply: 'ok' }), { status: 200 }),
  );
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => vi.unstubAllGlobals());

// A page load as the app does it: server-render, then hydrate. send() types a
// message, submits it and returns the session it was sent under.
async function loadPage() {
  const container = document.createElement('div');
  container.innerHTML = renderToString(<SearchExperience />);
  document.body.appendChild(container);
  const root = await act(async () =>
    hydrateRoot(container, <SearchExperience />),
  );
  return {
    async send(message: string) {
      fetchMock.mockClear();
      fireEvent.change(container.querySelector('#property-query')!, {
        target: { value: message },
      });
      fireEvent.submit(container.querySelector('form.query-form')!);
      await waitFor(() =>
        expect(fetchMock).toHaveBeenCalledWith('/api/agent', expect.anything()),
      );
      const [, init] = fetchMock.mock.calls.find(
        ([url]) => url === '/api/agent',
      )!;
      // Let the reply land so the form is idle for the next turn.
      await act(async () => {});
      return JSON.parse(init.body).session_id as string;
    },
    async startOver() {
      const button = [...container.querySelectorAll('button')].find(
        (b) => b.textContent?.trim() === 'Empezar de nuevo',
      );
      expect(button, 'a "Empezar de nuevo" button').toBeDefined();
      await act(async () => fireEvent.click(button!));
    },
    text: () => container.textContent ?? '',
    close() {
      act(() => root.unmount());
      container.remove();
    },
  };
}

describe('SearchExperience', () => {
  it('keeps the browser chat across a reload', async () => {
    const first = await loadPage();
    const firstSession = await first.send('Monoambiente cerca del subte A');
    first.close();

    const second = await loadPage();
    const secondSession = await second.send('Dos ambientes en Palermo');
    second.close();

    expect(secondSession).toBe(firstSession);
  });

  it('keeps one conversation across the turns of a page load', async () => {
    const page = await loadPage();
    const opening = await page.send('Dos ambientes en Palermo');
    const followUp = await page.send('Que tenga balcón');
    page.close();

    expect(followUp).toBe(opening);
  });

  it('starts a new conversation on "Empezar de nuevo"', async () => {
    const page = await loadPage();
    const before = await page.send('Monoambiente cerca del subte A');
    await page.startOver();
    expect(page.text()).not.toContain('Monoambiente cerca del subte A');
    const after = await page.send('Dos ambientes en Palermo');
    page.close();

    expect(after).not.toBe(before);
  });

  it('returns home from the logo and forgets the search', async () => {
    const page = await loadPage();
    const before = await page.send('Monoambiente cerca del subte A');
    await act(async () =>
      fireEvent.click(document.querySelector('a[aria-label="Hausy, inicio"]')!),
    );
    expect(page.text()).toContain('¿Qué estás buscando?');
    expect(page.text()).not.toContain('Monoambiente cerca del subte A');
    page.close();

    const reloaded = await loadPage();
    expect(reloaded.text()).toContain('¿Qué estás buscando?');
    const after = await reloaded.send('Dos ambientes en Palermo');
    reloaded.close();

    expect(after).not.toBe(before);
  });

  it('asks a typed clarification before showing results', async () => {
		fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ clarification: {
			id: 'q1', request: 'Busco dos habitaciones en Palermo', source: 'dos habitaciones',
			prompt: 'Cuando dijiste «dos habitaciones», ¿ambientes o dormitorios?', kind: 'search',
			choices: [{ id: 'ambientes', label: 'Dos ambientes' }, { id: 'dormitorios', label: 'Dos dormitorios' }],
		} }), { status: 200 }));
		const page = await loadPage();
		await page.send('Busco dos habitaciones en Palermo');
		expect(page.text()).toContain('Cuando dijiste «dos habitaciones»');
		expect(page.text()).toContain('Ninguna de estas');
		expect(document.querySelector('form.query-form')).toBeNull();
		// No results column yet, so the qualification stays reachable here.
		expect(page.text()).toContain('¿Qué garantía tenés?');
		fireEvent.click(document.querySelector('input[value="dormitorios"]')!);
		fireEvent.click([...document.querySelectorAll('button')].find((b) => b.textContent?.trim() === 'Continuar búsqueda')!);
		await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
		const answer = JSON.parse(fetchMock.mock.calls[1][1].body);
		expect(answer.answer).toEqual({ question_id: 'q1', selected: ['dormitorios'] });
		await waitFor(() => expect(page.text()).toContain('ok'));
		page.close();
	});

  it('shows the cards and the reply while the turn is still being written', async () => {
    const encoder = new TextEncoder();
    let pushText!: (text: string) => void;
    let end!: () => void;
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        pushText = (text) => controller.enqueue(encoder.encode(text));
        end = () => controller.close();
      },
    });
    const line = (event: object) => JSON.stringify(event) + '\n';
    const push = (event: object) => pushText(line(event));
    fetchMock.mockResolvedValueOnce(
      new Response(body, {
        headers: { 'Content-Type': 'application/x-ndjson' },
      }),
    );
    const listing = {
      rank: 1,
      source: 'zonaprop',
      url: 'https://zonaprop.com.ar/1',
      neighborhood: 'palermo',
      address: 'Humboldt 1900',
      description: 'Depto luminoso',
      price: { amount: 450000, currency: 'ARS' },
      expenses: { amount: null, currency: null },
    };
    const page = await loadPage();
    const sent = page.send('Dos ambientes en Palermo');

    const [, init] = await vi.waitFor(() => {
      const call = fetchMock.mock.calls.find(([url]) => url === '/api/agent');
      expect(call).toBeDefined();
      return call!;
    });
    expect(new Headers(init.headers).get('Accept')).toBe(
      'application/x-ndjson',
    );

    await act(async () => {
      push({
        type: 'results',
        reply: '',
        listings: [listing],
        requirements: [],
      });
      // Split mid-line on purpose: a chunk boundary is not an event boundary.
      const delta = line({
        type: 'reply',
        delta: '## Mi lectura\nEl #1 queda ',
      });
      pushText(delta.slice(0, 12));
      pushText(delta.slice(12));
    });
    await vi.waitFor(() => {
      expect(page.text()).toContain('Humboldt 1900');
      expect(page.text()).toContain('El #1 queda');
    });

    await act(async () => {
      push({ type: 'reply', delta: 'en Palermo.' });
      push({
        type: 'done',
        reply: '## Mi lectura\nEl #1 queda en Palermo, dentro de tu tope.',
        listings: [listing],
        requirements: [],
      });
      end();
    });
    await sent;
    await vi.waitFor(() => expect(page.text()).toContain('dentro de tu tope'));
    // The done reply is the authority: it replaces what was streamed.
    expect(page.text()).not.toContain('queda en Palermo.');
    page.close();
  });

  it('restores the pending question after a reload of the same browser chat', async () => {
    const question = { id: 'q-reload', request: 'dos habitaciones en Palermo', source: 'dos habitaciones', prompt: '¿Ambientes o dormitorios?', kind: 'search', choices: [{ id: 'a', label: 'Ambientes' }, { id: 'd', label: 'Dormitorios' }] };
    fetchMock.mockImplementation(async () => Response.json({ clarification: question }));
    const first = await loadPage();
    const sessionID = await first.send('dos habitaciones en Palermo');
    first.close();

    const second = await loadPage();
    await waitFor(() => expect(second.text()).toContain('¿Ambientes o dormitorios?'));
    expect(fetchMock).toHaveBeenCalledWith(`/api/agent?session_id=${encodeURIComponent(sessionID)}`, expect.anything());
    expect(document.querySelector('form.query-form')).toBeNull();
    second.close();
  });
});
