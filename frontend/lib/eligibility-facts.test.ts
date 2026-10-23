import { renderHook, waitFor } from '@testing-library/react';
import { createElement, type ReactNode } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { FactsContext, useEligibilityFacts } from './eligibility-facts';
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

  // Seen in a production build: the search page asked for the catalog once
  // per component that needed it.
  it('asks once for every component that needs the catalog at the same time', async () => {
    answer(() => Response.json({ facts: factsFixture }));
    const { result } = renderHook(() => [useEligibilityFacts(), useEligibilityFacts()]);
    await waitFor(() =>
      expect(result.current).toEqual([
        { kind: 'ready', facts: factsFixture },
        { kind: 'ready', facts: factsFixture },
      ]),
    );
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  // The page loads the catalog on the server, so the form renders with it.
  it('uses the catalog the page brought without asking', () => {
    answer(() => Response.json({ facts: [] }));
    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(FactsContext, { value: factsFixture }, children);
    const { result } = renderHook(() => useEligibilityFacts(), { wrapper });
    expect(result.current).toEqual({ kind: 'ready', facts: factsFixture });
    expect(fetch).not.toHaveBeenCalled();
  });
});
