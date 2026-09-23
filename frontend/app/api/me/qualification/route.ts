import { passThrough, sessionToken, unreachable, withBearer } from '@/lib/auth';

// The signed-in searcher's qualification (CONTEXT.md). Anonymous visitors keep
// theirs in the page for the session only.
function signedOut() {
  return Response.json({ error: 'Tenés que iniciar sesión.' }, { status: 401 });
}

export async function GET(request: Request) {
  const token = sessionToken(request);
  if (!token) return signedOut();
  try {
    return await passThrough(await withBearer('/api/me/qualification', token));
  } catch {
    return unreachable();
  }
}

export async function PUT(request: Request) {
  const token = sessionToken(request);
  if (!token) return signedOut();
  try {
    return await passThrough(
      await withBearer(
        '/api/me/qualification',
        token,
        'PUT',
        await request.text(),
      ),
    );
  } catch {
    return unreachable();
  }
}
