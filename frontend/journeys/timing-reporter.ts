import fs from 'node:fs';
import path from 'node:path';

import type {
  FullConfig,
  Reporter,
  TestCase,
  TestResult,
} from '@playwright/test/reporter';

import { summarize, toCsv, type Sample } from './timings';

const out = 'test-results/journeys';

// Reads each journey's top-level test.step durations. Every run writes
// journeys.json and prints a table; a run that repeats each journey
// (--repeat-each, as `npm run journeys:snapshot` does) also writes the dated
// snapshot under docs/metrics, since one sample per step is not a baseline.
// A failed run adds no samples, so it shows as fewer runs for its steps.
export default class TimingReporter implements Reporter {
  private samples: Sample[] = [];
  private repeated = false;

  onBegin(config: FullConfig) {
    this.repeated = config.projects.some((p) => p.repeatEach > 1);
  }

  onTestEnd(test: TestCase, result: TestResult) {
    if (result.status !== 'passed') return;
    for (const step of result.steps) {
      if (step.category !== 'test.step') continue;
      this.samples.push({
        journey: test.title,
        step: step.title,
        ms: step.duration,
      });
    }
  }

  onEnd() {
    const rows = summarize(this.samples);
    fs.mkdirSync(out, { recursive: true });
    fs.writeFileSync(
      path.join(out, 'journeys.json'),
      JSON.stringify({ samples: this.samples, steps: rows }, null, 2),
    );
    console.log('\nJourney step timings (fake model: app and database only)\n');
    console.table(
      rows.map((r) => ({
        journey: r.journey,
        step: r.step,
        runs: r.runs,
        'median ms': Math.round(r.median),
        'p95 ms': Math.round(r.p95),
      })),
    );
    if (this.repeated && rows.length > 0) {
      const date = new Date().toLocaleDateString('sv');
      const dir = path.join('..', 'docs', 'metrics', date);
      fs.mkdirSync(dir, { recursive: true });
      fs.writeFileSync(path.join(dir, 'journeys.csv'), toCsv(rows));
      console.log(`Snapshot written to ${path.join(dir, 'journeys.csv')}`);
    }
  }
}
