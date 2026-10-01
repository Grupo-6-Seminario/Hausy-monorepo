import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  THEME_STORAGE_KEY,
  applyTheme,
  currentTheme,
  themeBootScript,
} from './theme';

// Every test starts with no Storage, which is also what a browser with site
// data blocked looks like from the inside. Tests that need one install it; the
// rest exercise the page with none, the way a private window would. jsdom does
// provide a real one at an http origin, and it would carry a theme written by
// one test into the next.
function installStorage(): Storage {
  const entries = new Map<string, string>();
  const storage = {
    get length() {
      return entries.size;
    },
    key: (index: number) => [...entries.keys()][index] ?? null,
    getItem: (key: string) => entries.get(key) ?? null,
    setItem: (key: string, value: string) => void entries.set(key, value),
    removeItem: (key: string) => void entries.delete(key),
    clear: () => entries.clear(),
  } satisfies Storage;

  vi.stubGlobal('localStorage', storage);
  return storage;
}

/** Pins `prefers-color-scheme: dark` for the duration of a test. */
function systemPrefersDark(dark: boolean) {
  vi.stubGlobal(
    'matchMedia',
    vi.fn((query: string) => ({
      matches: query.includes('prefers-color-scheme: dark') && dark,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  );
}

beforeEach(() => {
  delete document.documentElement.dataset.theme;
  vi.stubGlobal('localStorage', undefined);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('currentTheme', () => {
  it('prefers an explicit choice over the system preference', () => {
    systemPrefersDark(true);
    document.documentElement.dataset.theme = 'light';

    expect(currentTheme()).toBe('light');
  });

  it('settles on light where the environment cannot be asked', () => {
    vi.stubGlobal('matchMedia', undefined);

    expect(currentTheme()).toBe('light');
  });

  it('ignores an unreadable attribute value', () => {
    systemPrefersDark(false);
    document.documentElement.dataset.theme = 'sepia';

    expect(currentTheme()).toBe('light');
  });
});

describe('applyTheme', () => {
  it('still switches the page when there is nowhere to remember it', () => {
    expect(() => applyTheme('dark')).not.toThrow();
    expect(document.documentElement.dataset.theme).toBe('dark');
  });
});

/*
 * The subject here is a string destined for a <script> tag, so running it is
 * the only test that proves anything. jsdom does not execute inline scripts,
 * which leaves the Function constructor.
 */
/* oxlint-disable typescript/no-implied-eval */
describe('themeBootScript', () => {
  it('ignores a stored value it does not recognise', () => {
    installStorage().setItem(THEME_STORAGE_KEY, 'sepia');

    new Function(themeBootScript)();

    expect(document.documentElement.dataset.theme).toBeUndefined();
  });

  it('leaves the document alone when storage is unavailable', () => {
    expect(() => new Function(themeBootScript)()).not.toThrow();
    expect(document.documentElement.dataset.theme).toBeUndefined();
  });
});
/* oxlint-enable typescript/no-implied-eval */
