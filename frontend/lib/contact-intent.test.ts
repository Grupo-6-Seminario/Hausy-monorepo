import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  recordContactIntent,
  stableListingID,
  type ContactIntentRecord,
} from './contact-intent';

afterEach(() => {
  vi.unstubAllGlobals();
});

const UUID_V4 =
  /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

function respond(status: number, record: Partial<ContactIntentRecord>) {
  return new Response(JSON.stringify(record), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function echoBackend(status = 201) {
  return vi.fn(async (_path: string, init: RequestInit) => {
    const sent = JSON.parse(init.body as string) as { intent_id: string };
    return respond(status, {
      intent_id: sent.intent_id,
      listing_id: '101',
      recorded: true,
    });
  });
}

describe('recordContactIntent', () => {
  it('posts a fresh UUID and the source to the listing contact-intents path', async () => {
    const fetchMock = echoBackend();
    vi.stubGlobal('fetch', fetchMock);

    const record = await recordContactIntent('101', 'search_result_card');

    const [path, init] = fetchMock.mock.calls[0];
    expect(path).toBe('/api/listings/101/contact-intents');
    expect(init.method).toBe('POST');

    const body = JSON.parse(init.body as string);
    expect(body.source).toBe('search_result_card');
    expect(body.intent_id).toMatch(UUID_V4);
    expect(Object.keys(body).sort()).toEqual(['intent_id', 'source']);

    expect(record).toEqual({
      intent_id: body.intent_id,
      listing_id: '101',
      recorded: true,
    });
  });

  it('mints a new intent id per activation so two events are never conflated', async () => {
    const fetchMock = echoBackend();
    vi.stubGlobal('fetch', fetchMock);

    const first = await recordContactIntent('101');
    const second = await recordContactIntent('101');

    expect(first.intent_id).not.toBe(second.intent_id);
  });

  it('accepts an idempotent replay answered with 200', async () => {
    vi.stubGlobal('fetch', echoBackend(200));

    await expect(recordContactIntent('101')).resolves.toMatchObject({
      recorded: true,
    });
  });

  it('escapes a listing id that would otherwise reshape the path', async () => {
    const fetchMock = echoBackend();
    vi.stubGlobal('fetch', fetchMock);

    await recordContactIntent('zonaprop/101');

    expect(fetchMock.mock.calls[0][0]).toBe(
      '/api/listings/zonaprop%2F101/contact-intents',
    );
  });

  it('rejects when the backend refuses the intent', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(respond(502, {})));

    await expect(recordContactIntent('101')).rejects.toThrow();
  });

  it('rejects when the backend answers about a different intent', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        respond(201, {
          intent_id: 'ffffffff-ffff-4fff-bfff-ffffffffffff',
          listing_id: '101',
          recorded: true,
        }),
      ),
    );

    await expect(recordContactIntent('101')).rejects.toThrow();
  });

  it('rejects when the backend says the intent was not recorded', async () => {
    const fetchMock = vi.fn(async (_path: string, init: RequestInit) => {
      const sent = JSON.parse(init.body as string) as { intent_id: string };
      return respond(201, {
        intent_id: sent.intent_id,
        listing_id: '101',
        recorded: false,
      });
    });
    vi.stubGlobal('fetch', fetchMock);

    await expect(recordContactIntent('101')).rejects.toThrow();
  });
});

describe('stableListingID', () => {
  it('reads the listing record id, numeric or string', () => {
    expect(stableListingID({ id: 101 })).toBe('101');
    expect(stableListingID({ id: 'zonaprop-101' })).toBe('zonaprop-101');
  });

  it('reports no stable id rather than inventing one', () => {
    expect(stableListingID({})).toBeUndefined();
    expect(stableListingID({ id: undefined })).toBeUndefined();
    expect(stableListingID({ id: '   ' })).toBeUndefined();
  });
});
