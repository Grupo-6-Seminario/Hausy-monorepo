import { afterEach, describe, expect, it, vi } from 'vitest';

import { POST } from './route';

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('POST /api/agent', () => {
  it('forwards the browser message to the local Hausy backend', async () => {
    const backendReply = {
      reply: 'Respuesta real del agente.',
      requirements: [],
    };
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(backendReply), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    vi.stubGlobal('fetch', fetchMock);

    const body = JSON.stringify({
      session_id: 'browser-session',
      message: 'Busco luz',
    });
    const request = new Request('http://localhost/api/agent', {
      method: 'POST',
      body,
      headers: { 'Content-Type': 'application/json' },
    });
    const response = await POST(request);

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/api/messages',
      expect.objectContaining({ method: 'POST', body }),
    );
    await expect(response.json()).resolves.toEqual(backendReply);
  });

  it('asks the backend to stream when the browser does, and passes the stream through', async () => {
    const stream =
      '{"type":"reply","delta":"Hola"}\n{"type":"done","reply":"Hola"}\n';
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(stream, {
        status: 200,
        headers: { 'Content-Type': 'application/x-ndjson; charset=utf-8' },
      }),
    );
    vi.stubGlobal('fetch', fetchMock);

    const response = await POST(
      new Request('http://localhost/api/agent', {
        method: 'POST',
        body: '{"message":"Palermo"}',
        headers: {
          'Content-Type': 'application/json',
          Accept: 'application/x-ndjson',
        },
      }),
    );

    const [, init] = fetchMock.mock.calls[0];
    expect(new Headers(init.headers).get('Accept')).toBe(
      'application/x-ndjson',
    );
    expect(response.headers.get('Content-Type')).toContain(
      'application/x-ndjson',
    );
    await expect(response.text()).resolves.toBe(stream);
  });
});
