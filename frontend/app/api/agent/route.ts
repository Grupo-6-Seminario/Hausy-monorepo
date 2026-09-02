const backendURL = (process.env.HAUSY_BACKEND_URL ?? 'http://127.0.0.1:8080').replace(
  /\/$/,
  '',
);

export async function POST(request: Request) {
  const body = await request.text();

  try {
    const response = await fetch(`${backendURL}/api/messages`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body,
      signal: AbortSignal.timeout(65_000),
    });
    return new Response(response.body, {
      status: response.status,
      headers: { 'Content-Type': response.headers.get('Content-Type') ?? 'application/json' },
    });
  } catch {
    return Response.json(
      { error: 'No pudimos conectarnos con el agente local.' },
      { status: 502 },
    );
  }
}
