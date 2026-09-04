import {
  effect,
  frame,
  surface,
  type Effect,
  type Gpu,
  type Surface,
} from 'vgpu';

import {
  POINTER_GLOW_RADIUS_CSS_PX,
  POINTER_SMOOTHING_SECONDS,
  approach,
  pointerAim,
  type PointerAim,
} from './pointer';
import promptLedWgsl from './prompt-led.wgsl';

const MAX_RENDER_WIDTH = 1100;
const MAX_RENDER_HEIGHT = 420;
const ACTIVE_FRAME_INTERVAL = 1000 / 60;
const IDLE_FRAME_INTERVAL = 1000 / 30;
const CANVAS_INSET_CSS_PX = 110;
// The emitter ring sits a hair outside the prompt's border box so the crisp
// front line stays clear of the translucent panel stacked above the canvas.
const RING_BIAS_CSS_PX = 3;
// How far the continuous strip is allowed to dip below full brightness.
const SHIMMER_DEPTH = 0.12;

function renderMetrics(canvas: HTMLCanvasElement) {
  const bounds = canvas.getBoundingClientRect();
  const scale = Math.min(
    1,
    MAX_RENDER_WIDTH / Math.max(1, bounds.width),
    MAX_RENDER_HEIGHT / Math.max(1, bounds.height),
  );
  return {
    size: [
      Math.max(160, Math.round(bounds.width * scale)),
      Math.max(96, Math.round(bounds.height * scale)),
    ] as const,
    scale,
  };
}

function uniforms(
  size: readonly [number, number],
  scale: number,
  time: number,
  pointer: PointerAim,
) {
  const inset = Math.max(24, CANVAS_INSET_CSS_PX * scale);
  return {
    frame: [size[0], size[1], time, inset],
    shape: [22 * scale, 1.6 * scale, SHIMMER_DEPTH, RING_BIAS_CSS_PX * scale],
    pointer: [
      pointer.x,
      pointer.y,
      pointer.strength,
      POINTER_GLOW_RADIUS_CSS_PX * scale,
    ],
    // DESIGN.md prism channels, kept inside the luminary artifact.
    colour_a: [1, 0.1647, 0.1647, 1],
    colour_b: [0.1647, 0.498, 1, 1],
    colour_c: [0.1647, 1, 0.1647, 1],
  };
}

const DARK: PointerAim = { x: -1e4, y: -1e4, strength: 0 };

export function createPromptLedBorder(canvas: HTMLCanvasElement) {
  let disposed = false;
  let gpu: Gpu | undefined;
  let output: Surface | undefined;
  let shader: Effect | undefined;
  let observer: ResizeObserver | undefined;
  let animationFrame = 0;
  let lastFrame = -IDLE_FRAME_INTERVAL;
  let size: readonly [number, number] = [1, 1];
  let renderScale = 1;
  let target: PointerAim = DARK;
  let current: PointerAim = DARK;

  const draw = (time: number) => {
    if (!gpu || !output || !shader) return;
    shader.set({ led: uniforms(size, renderScale, time, current) });
    frame(gpu, (currentFrame) => {
      currentFrame.pass({ target: output!, clear: [0, 0, 0, 0] }, (pass) =>
        pass.draw(shader!),
      );
    });
  };

  const resize = () => {
    if (!output) return;
    const metrics = renderMetrics(canvas);
    size = metrics.size;
    renderScale = metrics.scale;
    output.resize(size);
    draw(performance.now() / 1000);
  };

  // The canvas is inert (`pointer-events: none`) and sits under the prompt, so
  // the cursor is tracked on the window and projected into render space.
  const onPointerMove = (event: PointerEvent) => {
    if (event.pointerType !== 'mouse') return;
    target = pointerAim(
      canvas.getBoundingClientRect(),
      event.clientX,
      event.clientY,
      size,
    );
  };
  const onPointerOut = (event: PointerEvent) => {
    if (event.relatedTarget === null) target = { ...target, strength: 0 };
  };

  const tick = (timestamp: number) => {
    if (disposed) return;
    animationFrame = requestAnimationFrame(tick);
    const settled = current.strength < 0.002 && target.strength < 0.002;
    const interval = settled ? IDLE_FRAME_INTERVAL : ACTIVE_FRAME_INTERVAL;
    const elapsed = timestamp - lastFrame;
    if (document.hidden || elapsed < interval) return;
    const delta = Math.min(elapsed, 200) / 1000;
    lastFrame = timestamp;
    current = {
      x: approach(current.x, target.x, delta, POINTER_SMOOTHING_SECONDS),
      y: approach(current.y, target.y, delta, POINTER_SMOOTHING_SECONDS),
      strength: approach(
        current.strength,
        target.strength,
        delta,
        POINTER_SMOOTHING_SECONDS,
      ),
    };
    draw(timestamp / 1000);
  };

  const initialize = async () => {
    const { init } = await import('vgpu');
    const nextGpu = await init();
    if (disposed) {
      nextGpu.dispose();
      return;
    }
    gpu = nextGpu;
    output = surface(gpu, canvas, {
      alphaMode: 'premultiplied',
      autoResize: false,
    });
    shader = effect(gpu, promptLedWgsl);
    await shader.compile({ colors: [output.format] });
    if (disposed) return;
    resize();
    observer =
      typeof ResizeObserver === 'undefined'
        ? undefined
        : new ResizeObserver(resize);
    observer?.observe(canvas);
    window.addEventListener('pointermove', onPointerMove, { passive: true });
    window.addEventListener('pointerout', onPointerOut, { passive: true });
    if (!window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      animationFrame = requestAnimationFrame(tick);
    }
  };

  const ready = initialize();
  const dispose = () => {
    if (disposed) return;
    disposed = true;
    if (animationFrame) cancelAnimationFrame(animationFrame);
    window.removeEventListener('pointermove', onPointerMove);
    window.removeEventListener('pointerout', onPointerOut);
    observer?.disconnect();
    output?.dispose();
    gpu?.dispose();
  };

  return { ready, dispose };
}
