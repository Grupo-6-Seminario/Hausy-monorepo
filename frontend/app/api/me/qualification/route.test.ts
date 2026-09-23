import { afterEach, describe, expect, it, vi } from 'vitest';

import { GET, PUT } from './route';

afterEach(() => vi.unstubAllGlobals());

const withCookie = (method: string, body?: string) =>
  new Request('http://localhost/api/me/qualification', {
    method,
    body,
    headers: {
      Cookie: 'hausy_session=tok-1',
      'Content-Type': 'application/json',
    },
  });

describe('/api/me/qualification', () => {
  it('refuses anonymous visitors without calling the backend', async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    const response = await GET(
      new Request('http://localhost/api/me/qualification'),
    );
    expect(response.status).toBe(401);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('forwards reads and saves with the session as a bearer token', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(Response.json({ guarantee: ['caucion'] }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    vi.stubGlobal('fetch', fetchMock);

    const read = await GET(withCookie('GET'));
    expect(await read.json()).toEqual({ guarantee: ['caucion'] });
    const saved = await PUT(withCookie('PUT', '{"guarantee":["propietaria"]}'));
    expect(saved.status).toBe(204);
    expect(fetchMock).toHaveBeenLastCalledWith(
      'http://127.0.0.1:8080/api/me/qualification',
      expect.objectContaining({
        method: 'PUT',
        body: '{"guarantee":["propietaria"]}',
        headers: expect.objectContaining({ Authorization: 'Bearer tok-1' }),
      }),
    );
  });
});
