import { surface, type Gpu, type Surface } from 'vgpu';

import {
  createScene,
  destroyScene,
  prepareScene,
  presentScene,
  runChain,
  type RadianceScene,
} from './simulation';

const MAX_RENDER_WIDTH = 560;
const MAX_RENDER_HEIGHT = 220;
const FRAME_INTERVAL = 80;

function renderSize(canvas: HTMLCanvasElement): readonly [number, number] {
  const bounds = canvas.getBoundingClientRect();
  const scale = Math.min(
    0.72,
    MAX_RENDER_WIDTH / Math.max(1, bounds.width),
    MAX_RENDER_HEIGHT / Math.max(1, bounds.height),
  );
  return [
    Math.max(96, Math.round(bounds.width * scale)),
    Math.max(64, Math.round(bounds.height * scale)),
  ];
}

export function createRadianceBacklight(canvas: HTMLCanvasElement) {
  let disposed = false;
  let gpu: Gpu | undefined;
  let output: Surface | undefined;
  let scene: RadianceScene | undefined;
  let observer: ResizeObserver | undefined;
  let animationFrame = 0;
  let lastFrame = -FRAME_INTERVAL;
  let rebuilding = false;

  const draw = (time: number) => {
    if (!scene || !output) return;
    runChain(scene, time);
    presentScene(scene, output);
  };

  const rebuild = async () => {
    if (disposed || rebuilding || !gpu || !output) return;
    rebuilding = true;
    const size = renderSize(canvas);
    let next: RadianceScene | undefined;
    try {
      output.resize(size);
      next = createScene(gpu, size);
      await prepareScene(next, output.format);
      if (disposed) {
        destroyScene(next);
        return;
      }
      const previous = scene;
      scene = next;
      if (previous) destroyScene(previous);
      draw(performance.now() / 1000);
    } catch (error) {
      if (next && next !== scene) destroyScene(next);
      throw error;
    } finally {
      rebuilding = false;
    }
  };

  const tick = (timestamp: number) => {
    if (disposed) return;
    if (!document.hidden && timestamp - lastFrame >= FRAME_INTERVAL && !rebuilding) {
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
    await rebuild();
    if (disposed) return;

    if (typeof ResizeObserver !== 'undefined') {
      observer = new ResizeObserver(() => void rebuild());
      observer.observe(canvas);
    }
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
    if (scene) destroyScene(scene);
    gpu?.dispose();
  };

  return { ready, dispose };
}
