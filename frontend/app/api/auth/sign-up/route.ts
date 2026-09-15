import { passThrough, postToBackend, signIn, unreachable } from '@/lib/auth';

export async function POST(request: Request) {
  const body = await request.text();

  try {
    const created = await postToBackend('/api/auth/sign-up', body);
    if (!created.ok) return passThrough(created);

    // The backend accepted these exact fields, so they parse.
    const { email, password } = JSON.parse(body) as {
      email: string;
      password: string;
    };
    return await signIn(request, { email, password }, 201);
  } catch {
    return unreachable();
  }
}
