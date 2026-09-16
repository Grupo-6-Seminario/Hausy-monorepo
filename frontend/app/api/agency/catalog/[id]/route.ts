import { passThrough, sessionToken, unreachable, withBearer } from '@/lib/auth';

interface RouteContext {
  params: Promise<{ id: string }>;
}

function unauthenticated() {
  return Response.json({ error: 'Tenés que iniciar sesión.' }, { status: 401 });
}

export async function PATCH(request: Request, context: RouteContext) {
  const token = sessionToken(request);
  if (!token) return unauthenticated();
  const { id } = await context.params;

  try {
    return await passThrough(
      await withBearer(
        `/api/agency/catalog/${encodeURIComponent(id)}`,
        token,
        'PATCH',
        await request.text(),
      ),
    );
  } catch {
    return unreachable();
  }
}

export async function DELETE(request: Request, context: RouteContext) {
  const token = sessionToken(request);
  if (!token) return unauthenticated();
  const { id } = await context.params;

  try {
    return await passThrough(
      await withBearer(
        `/api/agency/catalog/${encodeURIComponent(id)}`,
        token,
        'DELETE',
      ),
    );
  } catch {
    return unreachable();
  }
}
