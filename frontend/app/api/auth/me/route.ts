import { passThrough, sessionToken, unreachable, withBearer } from '@/lib/auth';

export async function GET(request: Request) {
  const token = sessionToken(request);
  if (!token) {
    return Response.json(
      { error: 'Tenés que iniciar sesión.' },
      { status: 401 },
    );
  }

  try {
    return await passThrough(await withBearer('/api/auth/me', token));
  } catch {
    return unreachable();
  }
}
