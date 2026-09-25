import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  recordContactIntent,
  stableListingID,
  type ContactIntentRecord,
} from './contact-intent';

afterEach(() => {
  vi.unstubAllGlobals();
});

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
  it('reports no stable id rather than inventing one', () => {
    expect(stableListingID({})).toBeUndefined();
    expect(stableListingID({ id: undefined })).toBeUndefined();
    expect(stableListingID({ id: '   ' })).toBeUndefined();
  });
});
