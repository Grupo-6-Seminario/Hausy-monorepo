'use client';

import { RefObject, useEffect } from 'react';

/**
 * Lights `[data-glow]` controls inside `root` where the pointer rests.
 * One delegated `pointerover` listens on the root; `pointermove` is attached
 * only to the hovered control. Position lives in `--glow-x`/`--glow-y` CSS
 * variables, never React state. Touch pointers are ignored.
 */
export function usePointerGlow(root: RefObject<HTMLElement | null>) {
  useEffect(() => {
    const element = root.current;
    if (!element) return;

    let target: HTMLElement | null = null;

    const track = (event: PointerEvent) => {
      if (!target) return;
      const box = target.getBoundingClientRect();
      target.style.setProperty('--glow-x', `${event.clientX - box.left}px`);
      target.style.setProperty('--glow-y', `${event.clientY - box.top}px`);
    };

    const release = () => {
      target?.removeEventListener('pointermove', track);
      target = null;
    };

    const enter = (event: PointerEvent) => {
      if (event.pointerType === 'touch') return;
      const next =
        event.target instanceof Element
          ? event.target.closest<HTMLElement>('[data-glow]')
          : null;
      if (next !== target) {
        release();
        if (!next) return;
        target = next;
        target.addEventListener('pointermove', track);
      }
      track(event);
    };

    element.addEventListener('pointerover', enter);
    return () => {
      element.removeEventListener('pointerover', enter);
      release();
    };
  }, [root]);
}
