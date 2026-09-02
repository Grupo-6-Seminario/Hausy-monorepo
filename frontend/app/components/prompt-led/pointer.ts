// Pointer plumbing for the LED border. The upstream triangle-led-front example
// lifts the brightness of every emitter within a radius of the mouse and eases
// that lift with an exponential smoother; these helpers are the same model,
// expressed against the prompt canvas instead of the triangle simulation.

export interface CanvasRect {
  readonly left: number;
  readonly top: number;
  readonly width: number;
  readonly height: number;
}

export interface PointerAim {
  /** Pointer position in the canvas' render space, tracked even when outside. */
  readonly x: number;
  readonly y: number;
  /** How much the emitters should react, 1 over the canvas, 0 past the falloff. */
  readonly strength: number;
}

/** Distance, in CSS pixels beyond the canvas, over which the glow fades out. */
export const POINTER_FALLOFF_CSS_PX = 160;

/** Time constant of the glow smoother, matching the upstream `glowSmoothing`. */
export const POINTER_SMOOTHING_SECONDS = 0.23;

/** Radius of the pointer glow along the perimeter, in CSS pixels. */
export const POINTER_GLOW_RADIUS_CSS_PX = 220;

function smoothstep(edge0: number, edge1: number, value: number) {
  if (edge1 <= edge0) return value < edge0 ? 0 : 1;
  const t = Math.min(1, Math.max(0, (value - edge0) / (edge1 - edge0)));
  return t * t * (3 - 2 * t);
}

export function pointerAim(
  rect: CanvasRect,
  clientX: number,
  clientY: number,
  size: readonly [number, number],
): PointerAim {
  const width = Math.max(1, rect.width);
  const height = Math.max(1, rect.height);
  const localX = clientX - rect.left;
  const localY = clientY - rect.top;
  const overflowX = Math.max(0, -localX, localX - width);
  const overflowY = Math.max(0, -localY, localY - height);
  const overflow = Math.hypot(overflowX, overflowY);

  return {
    x: (localX / width) * size[0],
    y: (localY / height) * size[1],
    strength: 1 - smoothstep(0, POINTER_FALLOFF_CSS_PX, overflow),
  };
}

/** Exponential ease towards `target`, framerate independent. */
export function approach(
  current: number,
  target: number,
  deltaSeconds: number,
  tau: number,
) {
  if (tau <= 0) return target;
  return current + (target - current) * (1 - Math.exp(-deltaSeconds / tau));
}
