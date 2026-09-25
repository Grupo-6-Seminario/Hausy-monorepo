import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ThemeToggle } from './theme-toggle';

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
  systemPrefersDark(false);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('ThemeToggle', () => {
  it('switches back on a second press', async () => {
    installStorage();
    const user = userEvent.setup();
    render(<ThemeToggle />);

    await user.click(screen.getByRole('button'));
    await user.click(screen.getByRole('button'));

    expect(document.documentElement.dataset.theme).toBe('light');
  });

  it('names the theme the next press will bring', async () => {
    installStorage();
    const user = userEvent.setup();
    render(<ThemeToggle />);

    expect(
      await screen.findByRole('button', { name: 'Cambiar al tema oscuro' }),
    ).toBeInTheDocument();

    await user.click(screen.getByRole('button'));

    expect(
      screen.getByRole('button', { name: 'Cambiar al tema claro' }),
    ).toBeInTheDocument();
  });
});
