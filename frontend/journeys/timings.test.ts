import { describe, expect, it } from 'vitest';

import { summarize, toCsv } from './timings';

describe('journey timings', () => {
  it('summarizes each step by median and nearest-rank p95', () => {
    const rows = summarize([
      { journey: 'Realtor', step: 'Sign in', ms: 120 },
      { journey: 'Searcher', step: 'Search', ms: 300 },
      { journey: 'Searcher', step: 'Search', ms: 100 },
      { journey: 'Searcher', step: 'Search', ms: 200 },
      { journey: 'Searcher', step: 'Search', ms: 900 },
    ]);

    expect(rows).toEqual([
      { journey: 'Realtor', step: 'Sign in', runs: 1, median: 120, p95: 120 },
      { journey: 'Searcher', step: 'Search', runs: 4, median: 250, p95: 900 },
    ]);
  });

  it('writes the snapshot CSV with a header that names the fake model', () => {
    expect(
      toCsv([
        {
          journey: 'Searcher, new',
          step: 'Search',
          runs: 5,
          median: 250.4,
          p95: 900,
        },
      ]),
    ).toBe(
      '# Fake model: these times are app and database overhead, not model latency.\n' +
        'journey,step,runs,median_ms,p95_ms\n' +
        '"Searcher, new",Search,5,250,900\n',
    );
  });
});
