// Server-side helpers for the /api/auth/* routes. The backend hands out a bearer
// token; these routes keep it in an HttpOnly cookie so browser scripts never see
// it, and translate the cookie back into an Authorization header.

export type AuthRole = 'searcher' | 'realtor';

export interface AuthUser {
  id: string;
  email: string;
  name: string;
  role: AuthRole;
}

export const backendURL = (
  process.env.HAUSY_BACKEND_URL ?? 'http://127.0.0.1:8080'
).replace(/\/$/, '');

export const SESSION_COOKIE = 'hausy_session';

const timeout = () => AbortSignal.timeout(15_000);

export function sessionToken(request: Request): string | undefined {
  for (const part of (request.headers.get('Cookie') ?? '').split(';')) {
    const [name, ...value] = part.trim().split('=');
    if (name === SESSION_COOKIE && value.length > 0) {
      return decodeURIComponent(value.join('=')) || undefined;
    }
  }
  return undefined;
}

function cookieAttributes(request: Request) {
  const secure = new URL(request.url).protocol === 'https:' ? '; Secure' : '';
  return `Path=/; HttpOnly; SameSite=Lax${secure}`;
}

export function clearedSessionCookie(request: Request) {
  return `${SESSION_COOKIE}=; Max-Age=0; ${cookieAttributes(request)}`;
}

export function unreachable() {
  return Response.json(
    {
      error: 'No pudimos conectarnos con Hausy. Probá de nuevo en un momento.',
    },
    { status: 502 },
  );
}

export async function passThrough(response: Response) {
  const body = [204, 205, 304].includes(response.status)
    ? null
    : await response.text();
  return new Response(body, {
    status: response.status,
    headers: {
      'Content-Type':
        response.headers.get('Content-Type') ?? 'application/json',
    },
  });
}

export function postToBackend(path: string, body: string) {
  return fetch(`${backendURL}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body,
    signal: timeout(),
  });
}

export function withBearer(
  path: string,
  token: string,
  method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE' = 'GET',
  body?: string,
) {
  if (method === 'GET') {
    return fetch(`${backendURL}${path}`, {
      method: 'GET',
      headers: { Authorization: `Bearer ${token}` },
      signal: timeout(),
    });
  }
  return fetch(`${backendURL}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${token}`,
      ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
    },
    ...(body === undefined ? {} : { body }),
    signal: timeout(),
  });
}

// Signs in against the backend and answers with the user plus the session
// cookie. The token itself never reaches the response body.
export async function signIn(
  request: Request,
  credentials: { email: string; password: string },
  status = 200,
) {
  const response = await postToBackend(
    '/api/auth/sign-in',
    JSON.stringify(credentials),
  );
  if (!response.ok) return passThrough(response);

  const session = (await response.json()) as {
    token: string;
    expires_at: string;
    user: AuthUser;
  };
  const expires = new Date(session.expires_at).toUTCString();
  return Response.json(
    { user: session.user },
    {
      status,
      headers: {
        'Set-Cookie': `${SESSION_COOKIE}=${encodeURIComponent(session.token)}; Expires=${expires}; ${cookieAttributes(request)}`,
      },
    },
  );
}
