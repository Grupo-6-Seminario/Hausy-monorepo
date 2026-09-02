import { describe, expect, it } from 'vitest';

import { POINTER_FALLOFF_CSS_PX, approach, pointerAim } from './pointer';

const rect = { left: 100, top: 50, width: 800, height: 400 };
const size = [400, 200] as const;

describe('pointerAim', () => {
  it('maps a client position to render space and lights fully inside the canvas', () => {
    const aim = pointerAim(rect, 500, 250, size);

    expect(aim).toEqual({ x: 200, y: 100, strength: 1 });
  });

  it('keeps tracking past the edge and drops to darkness beyond the falloff', () => {
    const beyond = pointerAim(
      rect,
      rect.left + rect.width + POINTER_FALLOFF_CSS_PX,
      rect.top + rect.height / 2,
      size,
    );

    expect(beyond.x).toBeGreaterThan(size[0]);
    expect(beyond.strength).toBe(0);
  });

  it('dims gradually while the pointer approaches from outside', () => {
    const near = pointerAim(
      rect,
      rect.left + rect.width + POINTER_FALLOFF_CSS_PX / 2,
      rect.top + rect.height / 2,
      size,
    );

    expect(near.strength).toBeGreaterThan(0);
    expect(near.strength).toBeLessThan(1);
  });
});

describe('approach', () => {
  it('covers 1 - 1/e of the remaining gap over one time constant', () => {
    expect(approach(0, 1, 0.23, 0.23)).toBeCloseTo(0.6321, 4);
  });

  it('snaps to the target when there is no smoothing', () => {
    expect(approach(0, 1, 0.016, 0)).toBe(1);
  });
});
