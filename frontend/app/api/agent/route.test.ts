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
});
