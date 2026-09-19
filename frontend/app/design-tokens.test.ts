import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * Token contract between `app/globals.css` and `design-tokens.json`.
 *
 * Expected values are transcribed from the palette tables in `frontend/DESIGN.md`,
 * never read back out of the file under test. A test that recomputed them from
 * globals.css would pass against any value the stylesheet happened to hold.
 */

// jsdom's global URL resolves relative refs against the document base, not the
// file:// base, so these paths are built from the module path instead.
const here = path.dirname(fileURLToPath(import.meta.url));

const css = readFileSync(path.join(here, 'globals.css'), 'utf8');
const tokens = JSON.parse(
  readFileSync(path.join(here, '..', 'design-tokens.json'), 'utf8'),
) as { color: Record<'light' | 'dark', Record<string, { $value: string }>> };

type Theme = 'light' | 'dark';

/** Returns the contents of the `{...}` whose opening brace is at `open`. */
function braceBlock(source: string, open: number): string {
  let depth = 0;
  for (let i = open; i < source.length; i += 1) {
    if (source[i] === '{') depth += 1;
    else if (source[i] === '}') {
      depth -= 1;
      if (depth === 0) return source.slice(open + 1, i);
    }
  }
  throw new Error('unbalanced braces in globals.css');
}

function themeBlock(theme: Theme): string {
  if (theme === 'light') {
    // The light `:root` is at column 0; the dark one is nested and indented.
    const match = /^:root\s*\{/m.exec(css);
    if (!match) throw new Error('no top-level :root block');
    return braceBlock(css, match.index + match[0].length - 1);
  }
  const media = /@media\s*\(prefers-color-scheme:\s*dark\)\s*\{/.exec(css);
  if (!media) throw new Error('no prefers-color-scheme: dark block');
  const inner = braceBlock(css, media.index + media[0].length - 1);
  // The system route opts out when the reader has explicitly chosen light.
  const root = /:root:not\(\[data-theme=['"]light['"]\]\)\s*\{/.exec(inner);
  if (!root) throw new Error('no guarded :root inside the dark media query');
  return braceBlock(inner, root.index + root[0].length - 1);
}

/** The dark palette as applied by an explicit choice, not by the system. */
function chosenDarkBlock(): string {
  const match = /^:root\[data-theme=['"]dark['"]\]\s*\{/m.exec(css);
  if (!match) throw new Error('no :root[data-theme="dark"] block');
  return braceBlock(css, match.index + match[0].length - 1);
}

function declarations(theme: Theme): Map<string, string> {
  const found = new Map<string, string>();
  for (const [, name, value] of themeBlock(theme).matchAll(
    /(--[\w-]+)\s*:\s*([^;]+);/g,
  )) {
    found.set(name, value.trim());
  }
  return found;
}

const THEMES: Theme[] = ['light', 'dark'];

const SIGNAL_TOKENS: Record<Theme, Record<string, string>> = {
  light: {
    '--signal-conditional': '#8a5a18',
    '--signal-conditional-surface': '#f0e0bd',
    '--signal-evidence': '#1f5f63',
    '--signal-evidence-surface': '#d3e7e6',
    '--signal-unknown': '#5f6660',
    '--signal-unknown-surface': '#e4e6e0',
  },
  dark: {
    '--signal-conditional': '#e0b464',
    '--signal-conditional-surface': '#523f1e',
    '--signal-evidence': '#6cc2bd',
    '--signal-evidence-surface': '#1d4749',
    '--signal-unknown': '#aeb6ae',
    '--signal-unknown-surface': '#3a423c',
  },
};

const CHART_RAMP: Record<Theme, string[]> = {
  light: ['#a5d7ba', '#d49b4a', '#2f8d96', '#336d46', '#3d443c'],
  dark: ['#c0e5d0', '#e0b579', '#45acb5', '#47905f', '#5d6d5a'],
};

/** Raised card/popover surfaces, widened away from `--background`. */
const CARD_SURFACE: Record<Theme, string> = {
  light: '#fdfefa',
  dark: '#1c3b28',
};

/** `design-tokens.json` key -> the CSS custom property it mirrors. */
const JSON_TO_CSS: Record<string, string> = {
  background: '--background',
  foreground: '--foreground',
  card: '--card',
  primary: '--primary',
  secondary: '--secondary',
  muted: '--muted',
  mutedText: '--muted-foreground',
  accent: '--accent',
  border: '--border',
  input: '--input',
  ring: '--ring',
  error: '--destructive',
  signalConditional: '--signal-conditional',
  signalConditionalSurface: '--signal-conditional-surface',
  signalEvidence: '--signal-evidence',
  signalEvidenceSurface: '--signal-evidence-surface',
  signalUnknown: '--signal-unknown',
  signalUnknownSurface: '--signal-unknown-surface',
};

describe('semantic signal tokens', () => {
  it.each(THEMES)('defines every signal token in the %s theme', (theme) => {
    const declared = declarations(theme);

    for (const [token, value] of Object.entries(SIGNAL_TOKENS[theme])) {
      expect(declared.get(token), `${token} (${theme})`).toBe(value);
    }
  });

  it('defines the same signal tokens in both themes, with none one-sided', () => {
    const names = (theme: Theme) =>
      [...declarations(theme).keys()]
        .filter((name) => name.startsWith('--signal-'))
        .sort();

    expect(names('light')).toEqual(names('dark'));
    expect(names('light')).toEqual(Object.keys(SIGNAL_TOKENS.light).sort());
  });
});

describe('chart ramp', () => {
  it.each(THEMES)('uses the %s hue- and luminance-separated ramp', (theme) => {
    const declared = declarations(theme);

    for (const [index, value] of CHART_RAMP[theme].entries()) {
      expect(declared.get(`--chart-${index + 1}`), `--chart-${index + 1}`).toBe(
        value,
      );
    }
  });

  it.each(THEMES)(
    'keeps the five %s chart tokens pairwise distinct',
    (theme) => {
      const declared = declarations(theme);
      const ramp = [1, 2, 3, 4, 5].map((n) => declared.get(`--chart-${n}`));

      expect(ramp.every(Boolean)).toBe(true);
      expect(new Set(ramp).size).toBe(5);
    },
  );
});

describe('surface ladder', () => {
  it.each(THEMES)(
    'raises card, popover, and sidebar together in the %s theme',
    (theme) => {
      const declared = declarations(theme);

      for (const token of ['--card', '--popover', '--sidebar']) {
        expect(declared.get(token), `${token} (${theme})`).toBe(
          CARD_SURFACE[theme],
        );
      }
    },
  );
});

/*
 * CSS cannot union a media condition with a selector condition in one rule, so
 * the dark palette is written twice: once for the system preference and once
 * for a deliberate choice. This is the test that keeps the second copy honest.
 */
describe('chosen dark theme', () => {
  it('declares exactly what the system dark theme declares', () => {
    const chosen = new Map<string, string>();
    for (const [, name, value] of chosenDarkBlock().matchAll(
      /(--[\w-]+)\s*:\s*([^;]+);/g,
    )) {
      chosen.set(name, value.trim());
    }

    expect([...chosen.keys()].sort()).toEqual(
      [...declarations('dark').keys()].sort(),
    );
    expect(chosen).toEqual(declarations('dark'));
  });
});

describe('globals.css and design-tokens.json', () => {
  it.each(THEMES)('agree on every shared %s color value', (theme) => {
    const declared = declarations(theme);

    for (const [key, entry] of Object.entries(tokens.color[theme])) {
      const cssName = JSON_TO_CSS[key];
      expect(
        cssName,
        `design-tokens.json color.${theme}.${key} is unmapped`,
      ).toBeDefined();
      expect(entry.$value, `color.${theme}.${key} vs ${cssName}`).toBe(
        declared.get(cssName),
      );
    }
  });

  it.each(THEMES)('mirror every signal token into color.%s', (theme) => {
    const mirrored = tokens.color[theme];

    for (const [key, cssName] of Object.entries(JSON_TO_CSS)) {
      if (!cssName.startsWith('--signal-')) continue;
      expect(mirrored[key], `color.${theme}.${key}`).toEqual({
        $type: 'color',
        $value: SIGNAL_TOKENS[theme][cssName],
      });
    }
  });
});
