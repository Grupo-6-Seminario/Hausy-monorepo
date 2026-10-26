// Step timings for the journeys suite: the reporter collects one sample per
// step per run, and these reduce them to the snapshot docs/metrics keeps.

export type Sample = { journey: string; step: string; ms: number };
export type Row = {
  journey: string;
  step: string;
  runs: number;
  median: number;
  p95: number;
};

// summarize groups samples by journey and step, in the order they first ran.
export function summarize(samples: Sample[]): Row[] {
  const groups = Map.groupBy(samples, (s) => `${s.journey}\u0000${s.step}`);
  return [...groups.values()].map((group) => {
    const ms = group.map((s) => s.ms).sort((a, b) => a - b);
    const middle = Math.floor(ms.length / 2);
    return {
      journey: group[0].journey,
      step: group[0].step,
      runs: ms.length,
      median: ms.length % 2 ? ms[middle] : (ms[middle - 1] + ms[middle]) / 2,
      p95: ms[Math.ceil(0.95 * ms.length) - 1],
    };
  });
}

const field = (value: string | number) =>
  typeof value === 'string' && /[",\n]/.test(value)
    ? `"${value.replaceAll('"', '""')}"`
    : String(value);

export function toCsv(rows: Row[]): string {
  return [
    '# Fake model: these times are app and database overhead, not model latency.',
    'journey,step,runs,median_ms,p95_ms',
    ...rows.map((r) =>
      [r.journey, r.step, r.runs, Math.round(r.median), Math.round(r.p95)]
        .map(field)
        .join(','),
    ),
  ]
    .join('\n')
    .concat('\n');
}
