// Light/dark theme choice.
//
// The palette itself lives in `app/globals.css`. There are two ways a page can
// end up dark: the system asks for it, or the reader did. The system route is
// pure CSS and needs nothing from here. This module owns the second one: it
// writes `data-theme` on `<html>`, which outranks the media query, and
// remembers the choice so the next page load opens the same way.
//
// `themeBootScript` runs before first paint so a remembered dark page never
// flashes light. It is the same contract as `applyTheme`, written small enough
// to inline in the document head.

export type Theme = 'light' | 'dark';

/** Where a deliberate choice is remembered between visits. */
export const THEME_STORAGE_KEY = 'hausy-theme';

const SYSTEM_DARK = '(prefers-color-scheme: dark)';

function isTheme(value: string | undefined | null): value is Theme {
  return value === 'light' || value === 'dark';
}

/**
 * The theme the page is rendering in, chosen or inherited from the system.
 * Where there is no `matchMedia` to ask, this reads light, which is the same
 * thing the stylesheet falls back to.
 */
export function currentTheme(): Theme {
  const chosen = document.documentElement.dataset.theme;
  if (isTheme(chosen)) return chosen;
  return window.matchMedia?.(SYSTEM_DARK).matches ? 'dark' : 'light';
}

/**
 * Switches the page and remembers the choice. Storage can refuse the write in
 * a private window; the page still switches, it just forgets by the next load.
 */
export function applyTheme(theme: Theme): void {
  document.documentElement.dataset.theme = theme;
  try {
    localStorage.setItem(THEME_STORAGE_KEY, theme);
  } catch {
    // A page that will not remember is better than one that will not switch.
  }
}

/** Switches to the other theme and returns it. */
export function toggleTheme(): Theme {
  const next: Theme = currentTheme() === 'dark' ? 'light' : 'dark';
  applyTheme(next);
  return next;
}

/**
 * Applies a remembered choice before first paint. Inline it in the document
 * head, ahead of the body. Without a stored choice it touches nothing and the
 * media query in `globals.css` keeps the page on the system theme.
 */
export const themeBootScript = `try{var t=localStorage.getItem(${JSON.stringify(
  THEME_STORAGE_KEY,
)});if(t==="light"||t==="dark")document.documentElement.dataset.theme=t}catch(e){}`;
