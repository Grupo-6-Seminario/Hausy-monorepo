import { passThrough, sessionToken, unreachable, withBearer } from '@/lib/auth';

function unauthenticated() {
  return Response.json({ error: 'Tenés que iniciar sesión.' }, { status: 401 });
}

export async function GET(request: Request) {
  const token = sessionToken(request);
  if (!token) return unauthenticated();

  try {
    return await passThrough(await withBearer('/api/agency/catalog', token));
  } catch {
    return unreachable();
  }
}

export async function POST(request: Request) {
  const token = sessionToken(request);
  if (!token) return unauthenticated();

  try {
    return await passThrough(
      await withBearer(
        '/api/agency/catalog',
        token,
        'POST',
        await request.text(),
      ),
    );
  } catch {
    return unreachable();
  }
}
