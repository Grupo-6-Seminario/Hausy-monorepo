import { afterEach, describe, expect, it, vi } from 'vitest';

import { POST } from './route';

afterEach(() => {
  vi.unstubAllGlobals();
});

const intent = {
  intent_id: '3f6f9d1e-9b2a-4a4a-9a1e-1c2d3e4f5a6b',
  source: 'search_result_card',
};

function contactRequest(body: unknown = intent) {
  return new Request('http://localhost/api/listings/101/contact-intents', {
    method: 'POST',
    body: JSON.stringify(body),
    headers: { 'Content-Type': 'application/json' },
  });
}

function backendResponse(status: number, payload: unknown) {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('POST /api/listings/[listingId]/contact-intents', () => {
  it('forwards the intent to the backend under the listing id from the route', async () => {
    const recorded = {
      intent_id: intent.intent_id,
      listing_id: '101',
      recorded: true,
    };
    const fetchMock = vi.fn().mockResolvedValue(backendResponse(201, recorded));
    vi.stubGlobal('fetch', fetchMock);

    const response = await POST(contactRequest(), {
      params: Promise.resolve({ listingId: '101' }),
    });

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/api/listings/101/contact-intents',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify(intent),
      }),
    );
    expect(response.status).toBe(201);
    await expect(response.json()).resolves.toEqual(recorded);
  });

  it('never invents an agency, a user, or a counter in the forwarded body', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      backendResponse(201, {
        intent_id: intent.intent_id,
        listing_id: '101',
        recorded: true,
      }),
    );
    vi.stubGlobal('fetch', fetchMock);

    await POST(contactRequest(), {
      params: Promise.resolve({ listingId: '101' }),
    });

    const forwarded = JSON.parse(fetchMock.mock.calls[0][1].body as string);
    expect(Object.keys(forwarded).sort()).toEqual(['intent_id', 'source']);
  });

  it('passes an idempotent replay back unchanged', async () => {
    const replay = {
      intent_id: intent.intent_id,
      listing_id: '101',
      recorded: true,
    };
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(backendResponse(200, replay)),
    );

    const response = await POST(contactRequest(), {
      params: Promise.resolve({ listingId: '101' }),
    });

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toEqual(replay);
  });

  it('escapes the listing id instead of letting it reshape the backend path', async () => {
    const fetchMock = vi.fn().mockResolvedValue(backendResponse(201, {}));
    vi.stubGlobal('fetch', fetchMock);

    await POST(contactRequest(), {
      params: Promise.resolve({ listingId: '../../api/auth/sign-out' }),
    });

    expect(fetchMock.mock.calls[0][0]).toBe(
      'http://127.0.0.1:8080/api/listings/..%2F..%2Fapi%2Fauth%2Fsign-out/contact-intents',
    );
  });

  it('answers 502 when the backend is unreachable', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockRejectedValue(new Error('ECONNREFUSED')),
    );

    const response = await POST(contactRequest(), {
      params: Promise.resolve({ listingId: '101' }),
    });

    expect(response.status).toBe(502);
    await expect(response.json()).resolves.toHaveProperty('error');
  });
});
