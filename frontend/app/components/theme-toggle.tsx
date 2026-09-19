'use client';

import { Moon, Sun } from 'lucide-react';
import { useEffect, useRef } from 'react';

import { currentTheme, toggleTheme } from '@/lib/theme';

/**
 * Switches the page between the light and dark palettes.
 *
 * Which icon shows is decided in CSS, off the same cascade that picks the
 * palette, because the server renders this button without knowing the reader's
 * theme. Only the label needs JavaScript, and it is written straight to the
 * DOM after hydration, so no pointer-level state reaches React.
 */
export function ThemeToggle() {
  const buttonRef = useRef<HTMLButtonElement>(null);

  function nameTheNextPress() {
    const button = buttonRef.current;
    if (!button) return;
    button.setAttribute(
      'aria-label',
      currentTheme() === 'dark'
        ? 'Cambiar al tema claro'
        : 'Cambiar al tema oscuro',
    );
  }

  useEffect(nameTheNextPress, []);

  return (
    <button
      ref={buttonRef}
      type="button"
      className="theme-toggle"
      aria-label="Cambiar el tema"
      data-glow
      onClick={() => {
        toggleTheme();
        nameTheNextPress();
      }}
    >
      <Sun data-theme-icon="light" aria-hidden="true" />
      <Moon data-theme-icon="dark" aria-hidden="true" />
    </button>
  );
}
