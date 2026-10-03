import { renderHook, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { useEligibilityFacts } from './eligibility-facts';
import { factsFixture } from './eligibility-facts.fixture';

afterEach(() => vi.unstubAllGlobals());

function answer(response: () => Response) {
  vi.stubGlobal(
    'fetch',
    vi.fn(() => Promise.resolve(response())),
  );
}

describe('useEligibilityFacts', () => {
  it('is ready with the catalog the backend serves', async () => {
    answer(() => Response.json({ facts: factsFixture }));
    const { result } = renderHook(() => useEligibilityFacts());
    await waitFor(() =>
      expect(result.current).toEqual({ kind: 'ready', facts: factsFixture }),
    );
  });

  // A failed catalog hides the form and the situation bar, so search goes on
  // without a qualification instead of asking no questions or wrong ones.
  it.each([
    ['a server error', () => Response.json({ error: 'x' }, { status: 500 })],
    ['a malformed body', () => Response.json({ facts: [{ name: 'pets' }] })],
    ['an empty catalog', () => Response.json({ facts: [] })],
  ])('fails on %s', async (_, response) => {
    answer(response);
    const { result } = renderHook(() => useEligibilityFacts());
    await waitFor(() => expect(result.current).toEqual({ kind: 'failed' }));
  });
});
