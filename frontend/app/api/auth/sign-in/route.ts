import { signIn, unreachable } from '@/lib/auth';

export async function POST(request: Request) {
  try {
    const { email, password } = (await request.json()) as {
      email: string;
      password: string;
    };
    return await signIn(request, { email, password });
  } catch {
    return unreachable();
  }
}
