const backendURL = (process.env.HAUSY_BACKEND_URL ?? 'http://127.0.0.1:8080').replace(
  /\/$/,
  '',
);

export async function GET(request: Request) {
  const sessionID = new URL(request.url).searchParams.get('session_id');
  if (!sessionID) return Response.json({ clarification: null });
  try {
    const response = await fetch(`${backendURL}/api/messages?session_id=${encodeURIComponent(sessionID)}`);
    return new Response(response.body, { status: response.status, headers: { 'Content-Type': 'application/json' } });
  } catch {
    return Response.json({ clarification: null });
  }
}

export async function POST(request: Request) {
  const body = await request.text();

  try {
    const response = await fetch(`${backendURL}/api/messages`, {
      method: 'POST',
      // Accept carries the browser's choice of a streamed (NDJSON) or whole reply.
      headers: {
        'Content-Type': 'application/json',
        Accept: request.headers.get('Accept') ?? 'application/json',
      },
      body,
      signal: AbortSignal.timeout(65_000),
    });
    const headers = new Headers({
      'Content-Type': response.headers.get('Content-Type') ?? 'application/json',
    });
    const requestID = response.headers.get('X-Request-ID');
    if (requestID) headers.set('X-Request-ID', requestID);
    return new Response(response.body, {
      status: response.status,
      headers,
    });
  } catch {
    return Response.json(
      { error: 'No pudimos conectarnos con el agente local.' },
      { status: 502 },
    );
  }
}
