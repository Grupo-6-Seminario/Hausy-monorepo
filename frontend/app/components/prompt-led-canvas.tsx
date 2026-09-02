'use client';

import { useEffect, useRef } from 'react';

export function PromptLedCanvas() {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || typeof navigator === 'undefined' || !('gpu' in navigator)) {
      if (canvas) canvas.dataset.fallback = 'true';
      return;
    }

    let dispose: (() => void) | undefined;
    let cancelled = false;

    void import('./prompt-led/renderer')
      .then(({ createPromptLedBorder }) => {
        if (cancelled) return;
        const renderer = createPromptLedBorder(canvas);
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

  return (
    <canvas
      ref={canvasRef}
      className="prompt-led-canvas"
      data-prompt-led-border
      aria-hidden="true"
    />
  );
}
