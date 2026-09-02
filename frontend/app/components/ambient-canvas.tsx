'use client';

import { useEffect, useRef } from 'react';

export function AmbientCanvas() {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || typeof navigator === 'undefined' || !('gpu' in navigator)) {
      return;
    }

    let dispose: (() => void) | undefined;
    let cancelled = false;

    void import('./radiance/renderer')
      .then(({ createRadianceBacklight }) => {
        if (cancelled) return;
        const renderer = createRadianceBacklight(canvas);
        dispose = renderer.dispose;
        return renderer.ready;
      })
      .catch(() => {
        canvas.dataset.fallback = 'true';
      });

    return () => {
      cancelled = true;
      dispose?.();
    };
  }, []);

  return <canvas ref={canvasRef} className="ambient-canvas" aria-hidden="true" />;
}
