'use client';

import { useEffect, useRef } from 'react';

export function PromptLuminary() {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || typeof navigator === 'undefined' || !('gpu' in navigator)) {
      if (canvas) canvas.dataset.fallback = 'true';
      return;
    }

    let dispose: (() => void) | undefined;
    let cancelled = false;

    void import('./prompt-luminary/renderer')
      .then(({ startPromptLuminary }) => {
        if (cancelled) return;
        dispose = startPromptLuminary(canvas);
      })
      .catch(() => {
        canvas.dataset.fallback = 'true';
      });

    return () => {
      cancelled = true;
      dispose?.();
    };
  }, []);

  return (
    <canvas
      ref={canvasRef}
      className="prompt-luminary"
      data-prompt-luminary
      aria-hidden="true"
    />
  );
}
