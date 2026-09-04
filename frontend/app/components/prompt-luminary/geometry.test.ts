import { describe, expect, it } from 'vitest';

import { promptBoxMetrics } from './geometry';

describe('prompt luminary geometry', () => {
  it('maps the prompt perimeter into render-space coordinates', () => {
    expect(
      promptBoxMetrics(
        { left: 10, top: 20, width: 1000, height: 300 },
        { left: 60, top: 70, width: 900, height: 160 },
        [1250, 375],
        20,
      ),
    ).toEqual({
      box: [625, 162.5, 562.5, 100],
      radius: 25,
    });
  });
});
