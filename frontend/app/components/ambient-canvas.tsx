'use client';

import { useEffect, useRef } from 'react';

import ambientShader from './ambient.wgsl?raw';

export function AmbientCanvas() {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || typeof navigator === 'undefined' || !('gpu' in navigator)) {
      return;
    }

    let cancelled = false;
    let dispose: (() => void) | undefined;

    async function start() {
      const { clock, effect, frame, frameLoop, init, surface } = await import('vgpu');
      if (cancelled || !canvas) return;

      const gpu = await init();
      if (cancelled) {
        gpu.dispose();
        return;
      }

      const target = surface(gpu, canvas, {
        alphaMode: 'premultiplied',
        dpr: [1, 1.5],
      });
      const time = clock(gpu);
      const ambient = effect(gpu, ambientShader, {
        set: {
          params: {
            time: 0,
            width: target.size[0],
            height: target.size[1],
          },
        },
      });
      const unsubscribe = target.onResize(({ width, height }) => {
        ambient.set({ params: { width, height } });
      });
      const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

      await ambient.compile(target);

      if (reducedMotion) {
        frame(gpu, (currentFrame) => currentFrame.pass(target, ambient));
        dispose = () => {
          unsubscribe();
          gpu.dispose();
        };
        return;
      }

      const loop = frameLoop(gpu, (currentFrame) => {
        ambient.set({ params: { time: time.time } });
        currentFrame.pass(target, ambient);
      });

      dispose = () => {
        loop.stop();
        unsubscribe();
        gpu.dispose();
      };
    }

    start().catch(() => {
      canvas.dataset.fallback = 'true';
    });

    return () => {
      cancelled = true;
      dispose?.();
    };
  }, []);

  return <canvas ref={canvasRef} className="ambient-canvas" aria-hidden="true" />;
}
