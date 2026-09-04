import { clock, effect, frame, frameLoop, init, surface } from 'vgpu';

import { promptBoxMetrics, type PromptBoxMetrics } from './geometry';
import promptLuminaryWgsl from './prompt-luminary.wgsl';

interface Point {
  x: number;
  y: number;
}

const RESTING_POINTER: Point = { x: 0.5, y: 0.64 };
const SMOOTHING_SECONDS = 0.18;

function approach(current: number, target: number, deltaSeconds: number) {
  return (
    current +
    (target - current) * (1 - Math.exp(-deltaSeconds / SMOOTHING_SECONDS))
  );
}

function pointerInCanvas(
  canvas: HTMLCanvasElement,
  event: PointerEvent,
): Point {
  const bounds = canvas.getBoundingClientRect();
  return {
    x: Math.min(1, Math.max(0, (event.clientX - bounds.left) / bounds.width)),
    y: Math.min(1, Math.max(0, (event.clientY - bounds.top) / bounds.height)),
  };
}

function uniforms(
  size: readonly [number, number],
  time: number,
  pointer: Point,
  hover: number,
  active: number,
  metrics: PromptBoxMetrics,
) {
  return {
    frame: [size[0], size[1], time, active],
    pointer: [pointer.x * size[0], pointer.y * size[1], hover, 0],
    box: metrics.box,
    shape: [metrics.radius, 0, 0, 0],
    deep_green: [0.2275, 0.4941, 0.3098, 1],
    cozy_green: [0.4353, 0.7451, 0.5686, 1],
    soft_green: [0.8275, 0.8784, 0.7216, 1],
  };
}

/**
 * Starts the vgpu luminary attached to the prompt. The returned function owns
 * every listener and GPU resource so React strict-mode remounts stay safe.
 */
export function startPromptLuminary(canvas: HTMLCanvasElement): () => void {
  const host = canvas.closest<HTMLElement>('[data-prompt-stage]');
  if (!host) {
    canvas.dataset.fallback = 'true';
    return () => undefined;
  }

  const control = host.querySelector<HTMLElement>('.query-control');
  if (!control) {
    canvas.dataset.fallback = 'true';
    return () => undefined;
  }

  let disposed = false;
  let gpu: Awaited<ReturnType<typeof init>> | undefined;
  let loop: ReturnType<typeof frameLoop> | undefined;
  let stopResize: (() => void) | undefined;
  let resizeObserver: ResizeObserver | undefined;
  let targetPointer = RESTING_POINTER;
  let currentPointer = RESTING_POINTER;
  let targetHover = 0;
  let currentHover = 0;
  let lastTime = performance.now();

  const onPointerEnter = (event: PointerEvent) => {
    if (event.pointerType && event.pointerType !== 'mouse') return;
    targetPointer = pointerInCanvas(canvas, event);
    targetHover = 1;
  };

  const onPointerMove = (event: PointerEvent) => {
    if (event.pointerType && event.pointerType !== 'mouse') return;
    targetPointer = pointerInCanvas(canvas, event);
  };

  const onPointerLeave = () => {
    targetPointer = RESTING_POINTER;
    targetHover = 0;
  };

  host.addEventListener('pointerenter', onPointerEnter);
  host.addEventListener('pointermove', onPointerMove, { passive: true });
  host.addEventListener('pointerleave', onPointerLeave);

  void (async () => {
    gpu = await init();
    if (disposed) return gpu.dispose();

    const output = surface(gpu, canvas, { dpr: [1, 1.25] });
    let metrics: PromptBoxMetrics = {
      box: [0, 0, 0, 0],
      radius: 0,
    };

    const measurePrompt = () => {
      const radius = Number.parseFloat(
        getComputedStyle(control).borderTopLeftRadius,
      );
      metrics = promptBoxMetrics(
        canvas.getBoundingClientRect(),
        control.getBoundingClientRect(),
        output.size,
        Number.isFinite(radius) ? radius : 20,
      );
    };

    measurePrompt();
    resizeObserver = new ResizeObserver(measurePrompt);
    resizeObserver.observe(control);
    const reducedMotion = window.matchMedia(
      '(prefers-reduced-motion: reduce)',
    ).matches;
    const luminary = effect(gpu, promptLuminaryWgsl, {
      label: 'Hausy prompt luminary',
      set: {
        luminary: uniforms(
          output.size,
          0,
          RESTING_POINTER,
          0,
          host.dataset.active === 'true' ? 1 : 0,
          metrics,
        ),
      },
    });

    const drawStill = () => {
      luminary.set({
        luminary: uniforms(
          output.size,
          0,
          RESTING_POINTER,
          0,
          host.dataset.active === 'true' ? 1 : 0,
          metrics,
        ),
      });
      frame(gpu!, (currentFrame) => {
        currentFrame.pass({ target: output, clear: [0, 0, 0, 0] }, (pass) =>
          pass.draw(luminary),
        );
      });
    };

    if (reducedMotion) {
      drawStill();
      stopResize = output.onResize(() => {
        measurePrompt();
        drawStill();
      });
      return;
    }

    stopResize = output.onResize(measurePrompt);

    const time = clock(gpu);
    loop = frameLoop(gpu, (currentFrame) => {
      const now = performance.now();
      const delta = Math.min((now - lastTime) / 1000, 0.1);
      lastTime = now;
      currentPointer = {
        x: approach(currentPointer.x, targetPointer.x, delta),
        y: approach(currentPointer.y, targetPointer.y, delta),
      };
      currentHover = approach(currentHover, targetHover, delta);
      luminary.set({
        luminary: uniforms(
          output.size,
          time.time,
          currentPointer,
          currentHover,
          host.dataset.active === 'true' ? 1 : 0,
          metrics,
        ),
      });
      currentFrame.pass({ target: output, clear: [0, 0, 0, 0] }, (pass) =>
        pass.draw(luminary),
      );
    });
  })().catch(() => {
    canvas.dataset.fallback = 'true';
  });

  return () => {
    disposed = true;
    host.removeEventListener('pointerenter', onPointerEnter);
    host.removeEventListener('pointermove', onPointerMove);
    host.removeEventListener('pointerleave', onPointerLeave);
    stopResize?.();
    resizeObserver?.disconnect();
    loop?.stop();
    gpu?.dispose();
  };
}
