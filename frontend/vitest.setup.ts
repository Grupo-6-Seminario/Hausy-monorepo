import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';

afterEach(cleanup);

// jsdom has no FontFaceSet or ResizeObserver; the header wordmark uses both.
Object.defineProperty(document, 'fonts', { value: new EventTarget() });
globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};
