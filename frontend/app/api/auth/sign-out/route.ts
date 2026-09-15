import { clearedSessionCookie, sessionToken, withBearer } from '@/lib/auth';

export async function POST(request: Request) {
  const token = sessionToken(request);
  if (token) {
    try {
      await withBearer('/api/auth/sign-out', token, 'POST');
    } catch {
      // The cookie is cleared regardless; the session then simply expires.
    }
  }

  return new Response(null, {
    status: 204,
    headers: { 'Set-Cookie': clearedSessionCookie(request) },
  });
}
