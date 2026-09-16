import { afterEach, describe, expect, it, vi } from 'vitest';

import { DELETE, PATCH } from './route';

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

const context = { params: Promise.resolve({ id: '14' }) };

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('/api/agency/catalog/[id]', () => {
  it('forwards property edits to the authenticated backend route', async () => {
    const fetchMock = vi.fn().mockResolvedValue(json({ id: '14' }));
    vi.stubGlobal('fetch', fetchMock);
    const body = JSON.stringify({ address: 'Humboldt al 2000' });

    const response = await PATCH(
      new Request('http://localhost/api/agency/catalog/14', {
        method: 'PATCH',
        body,
        headers: { Cookie: 'hausy_session=secret-token' },
      }),
      context,
    );

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/api/agency/catalog/14',
      expect.objectContaining({ method: 'PATCH', body }),
    );
    expect(response.status).toBe(200);
  });

  it('forwards catalog removal and preserves the empty response', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal('fetch', fetchMock);

    const response = await DELETE(
      new Request('http://localhost/api/agency/catalog/14', {
        method: 'DELETE',
        headers: { Cookie: 'hausy_session=secret-token' },
      }),
      context,
    );

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/api/agency/catalog/14',
      expect.objectContaining({ method: 'DELETE' }),
    );
    expect(response.status).toBe(204);
  });
});
