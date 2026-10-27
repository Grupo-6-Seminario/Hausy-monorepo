# Theme and brand

**User sees:**
- The sun/moon toggle in the header switches light and dark. The choice holds on the next visit and on other pages, applied before the page paints.
- The "hausy" wordmark condenses past the heading and returns at the top.
- Under reduced motion, the wordmark swaps without a transition.
- The prompt's luminary canvas falls back without WebGPU.

**Source:**
- `frontend/app/components/theme-toggle.tsx`, `brand-link.tsx`, `prompt-luminary.tsx`, `prompt-luminary/`
- `frontend/app/layout.tsx`

**Drive:**
1. Open `/` and click the toggle.
2. Reload and expect the same theme.
3. For reduced motion, create the page with `reducedMotion: 'reduce'`.
4. Screenshot both themes.

**Automated:** `frontend/e2e/theme.e2e.ts`, `frontend/e2e/brand.e2e.ts`, and `npm run test:gpu` for the luminary.
