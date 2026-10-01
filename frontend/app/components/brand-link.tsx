'use client';

import { useEffect, useRef } from 'react';

/**
 * The header wordmark. Once the page heading has scrolled under the sticky
 * header, "ausy" fades and the mark slides next to the "h"; back at the
 * heading, the full name returns. The state lives in a DOM attribute so
 * scrolling never re-renders React, and the heading is looked up on every
 * check because some pages only render it after loading.
 */
export function BrandLink({
  href,
  onClick,
}: {
  href: string;
  onClick?: () => void;
}) {
  const linkRef = useRef<HTMLAnchorElement>(null);

  useEffect(() => {
    const link = linkRef.current;
    const header = link?.closest('header');
    const tail = link?.querySelector<HTMLElement>('.brand-tail');
    if (!link || !header || !tail) return;
    let frame = 0;

    const update = () => {
      frame = 0;
      // The mark travels exactly the tail's width, which changes when fonts swap in.
      const width = `${tail.getBoundingClientRect().width}px`;
      if (link.style.getPropertyValue('--brand-tail') !== width) {
        link.style.setProperty('--brand-tail', width);
      }
      const heading = document.querySelector('h1');
      link.toggleAttribute(
        'data-condensed',
        heading !== null &&
          heading.getBoundingClientRect().bottom <=
            header.getBoundingClientRect().bottom,
      );
    };
    const schedule = () => {
      frame ||= requestAnimationFrame(update);
    };

    // Views can swap under a still scroll position; the body resizes when they do.
    const layout = new ResizeObserver(schedule);
    update();
    layout.observe(document.body);
    addEventListener('scroll', schedule, { passive: true });
    document.fonts.addEventListener('loadingdone', schedule);
    return () => {
      cancelAnimationFrame(frame);
      layout.disconnect();
      removeEventListener('scroll', schedule);
      document.fonts.removeEventListener('loadingdone', schedule);
    };
  }, []);

  return (
    // Full navigation on purpose: vinext only shims next/link inside Vite, not vitest.
    // oxlint-disable-next-line next/no-html-link-for-pages
    <a
      ref={linkRef}
      className="brand"
      href={href}
      aria-label="Hausy, inicio"
      onClick={onClick}
    >
      <span>
        h<span className="brand-tail">ausy</span>
      </span>
      <span className="brand-mark" aria-hidden="true" />
    </a>
  );
}
