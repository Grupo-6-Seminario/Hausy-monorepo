import { afterEach, describe, expect, it, vi } from 'vitest';

import { GET as me } from './me/route';
import { POST as signIn } from './sign-in/route';
import { POST as signOut } from './sign-out/route';
import { POST as signUp } from './sign-up/route';

const marta = {
  id: '7',
  email: 'marta@inmobiliaria.com',
  name: 'Marta',
  role: 'realtor',
};

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function post(path: string, body: unknown, cookie?: string) {
  return new Request(`http://localhost${path}`, {
    method: 'POST',
    body: JSON.stringify(body),
    headers: {
      'Content-Type': 'application/json',
      ...(cookie ? { Cookie: cookie } : {}),
    },
  });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('auth routes', () => {
  it('keeps the session token in an HttpOnly cookie instead of the response body', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      json({
        token: 'secret-token',
        expires_at: '2026-10-14T12:00:00Z',
        user: marta,
      }),
    );
    vi.stubGlobal('fetch', fetchMock);

    const credentials = { email: marta.email, password: 'alquileres-caba' };
    const response = await signIn(post('/api/auth/sign-in', credentials));

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/api/auth/sign-in',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify(credentials),
      }),
    );
    expect(response.status).toBe(200);
    const cookie = response.headers.get('Set-Cookie') ?? '';
    expect(cookie).toContain('hausy_session=secret-token');
    expect(cookie).toContain('HttpOnly');
    expect(cookie).toContain('SameSite=Lax');
    expect(cookie).toContain('Path=/');
    expect(cookie).toContain('Expires=Wed, 14 Oct 2026 12:00:00 GMT');
    const body = await response.text();
    expect(body).not.toContain('secret-token');
    expect(JSON.parse(body)).toEqual({ user: marta });
  });

  it('passes backend errors through without setting a cookie', async () => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockResolvedValue(
          json({ error: 'El email o la contraseña no son correctos.' }, 401),
        ),
    );

    const response = await signIn(
      post('/api/auth/sign-in', { email: marta.email, password: 'mala' }),
    );

    expect(response.status).toBe(401);
    expect(response.headers.get('Set-Cookie')).toBeNull();
    await expect(response.json()).resolves.toEqual({
      error: 'El email o la contraseña no son correctos.',
    });
  });

  it('signs a new account in right after creating it', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(json({ user: marta }, 201))
      .mockResolvedValueOnce(
        json({
          token: 'fresh-token',
          expires_at: '2026-10-14T12:00:00Z',
          user: marta,
        }),
      );
    vi.stubGlobal('fetch', fetchMock);

    const registration = {
      email: marta.email,
      password: 'alquileres-caba',
      name: 'Marta',
      role: 'realtor',
    };
    const response = await signUp(post('/api/auth/sign-up', registration));

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      'http://127.0.0.1:8080/api/auth/sign-up',
      expect.objectContaining({ body: JSON.stringify(registration) }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      'http://127.0.0.1:8080/api/auth/sign-in',
      expect.objectContaining({
        body: JSON.stringify({
          email: marta.email,
          password: 'alquileres-caba',
        }),
      }),
    );
    expect(response.status).toBe(201);
    expect(response.headers.get('Set-Cookie')).toContain(
      'hausy_session=fresh-token',
    );
    await expect(response.json()).resolves.toEqual({ user: marta });
  });

  it('forwards the session cookie to the backend as a bearer token', async () => {
    const fetchMock = vi.fn().mockResolvedValue(json({ user: marta }));
    vi.stubGlobal('fetch', fetchMock);

    const response = await me(
      new Request('http://localhost/api/auth/me', {
        headers: { Cookie: 'theme=dark; hausy_session=secret-token' },
      }),
    );

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/api/auth/me',
      expect.objectContaining({
        headers: { Authorization: 'Bearer secret-token' },
      }),
    );
    await expect(response.json()).resolves.toEqual({ user: marta });
  });

  it('answers 401 without calling the backend when there is no session', async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);

    const response = await me(new Request('http://localhost/api/auth/me'));

    expect(response.status).toBe(401);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('revokes the session and clears the cookie on sign-out', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal('fetch', fetchMock);

    const response = await signOut(
      post('/api/auth/sign-out', {}, 'hausy_session=secret-token'),
    );

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/api/auth/sign-out',
      expect.objectContaining({
        method: 'POST',
        headers: { Authorization: 'Bearer secret-token' },
      }),
    );
    expect(response.status).toBe(204);
    const cookie = response.headers.get('Set-Cookie') ?? '';
    expect(cookie).toContain('hausy_session=;');
    expect(cookie).toContain('Max-Age=0');
  });

  it('explains when the backend cannot be reached', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('offline')));

    const response = await signIn(
      post('/api/auth/sign-in', { email: marta.email, password: 'x' }),
    );

    expect(response.status).toBe(502);
    await expect(response.json()).resolves.toEqual({
      error: 'No pudimos conectarnos con Hausy. Probá de nuevo en un momento.',
    });
  });
});
