import { effect, frame, surface, type Effect, type Gpu, type Surface } from 'vgpu';

import promptLedWgsl from './prompt-led.wgsl';

const MAX_RENDER_WIDTH = 760;
const MAX_RENDER_HEIGHT = 320;
const FRAME_INTERVAL = 1000 / 30;
const CANVAS_INSET_CSS_PX = 56;

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

function uniforms(size: readonly [number, number], scale: number, time: number) {
  const inset = Math.max(18, CANVAS_INSET_CSS_PX * scale);
  return {
    frame: [size[0], size[1], time, inset],
    shape: [22 * scale, 2.2 * scale, 32, 0],
    colour_a: [0.165, 0.353, 0.227, 1],
    colour_b: [0.435, 0.745, 0.569, 1],
  };
}

export function createPromptLedBorder(canvas: HTMLCanvasElement) {
  let disposed = false;
  let gpu: Gpu | undefined;
  let output: Surface | undefined;
  let shader: Effect | undefined;
  let observer: ResizeObserver | undefined;
  let animationFrame = 0;
  let lastFrame = -FRAME_INTERVAL;
  let size: readonly [number, number] = [1, 1];
  let renderScale = 1;

  const draw = (time: number) => {
    if (!gpu || !output || !shader) return;
    shader.set({ led: uniforms(size, renderScale, time) });
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

  const tick = (timestamp: number) => {
    if (disposed) return;
    if (!document.hidden && timestamp - lastFrame >= FRAME_INTERVAL) {
      lastFrame = timestamp;
      draw(timestamp / 1000);
    }
    animationFrame = requestAnimationFrame(tick);
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
    observer = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(resize);
    observer?.observe(canvas);
    if (!window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      animationFrame = requestAnimationFrame(tick);
    }
  };

  const ready = initialize();
  const dispose = () => {
    if (disposed) return;
    disposed = true;
    if (animationFrame) cancelAnimationFrame(animationFrame);
    observer?.disconnect();
    output?.dispose();
    gpu?.dispose();
  };

  return { ready, dispose };
}
