# Clarification of an unresolved condition

**User sees:** a search with a condition that has several plausible meanings, such as "con amenities", stops before results. It shows a heading "Cuando dijiste «con amenities»…" with choices. The question survives a reload. After the searcher checks a choice and clicks "Continuar búsqueda", a chip such as "Con pileta" joins "Tu búsqueda". Listings whose ad confirms the choice appear under "Coincidencias con evidencia", quoting it. The rest appear under "Otras opciones por confirmar".

**Source:**
- `internal/clarification`
- `internal/intake`
- ADR `docs/adr/0003-typed-clarification-before-search.md`
- `frontend/app/components/search-experience.tsx`

**Drive:** start anonymously at `/`.
1. Search 'Busco alquiler en Palermo con amenities'.
2. Expect the heading.
3. Reload and expect the heading again.
4. Check "Pileta" and click "Continuar búsqueda".
5. Expect the Cerviño al 4700 card under "Coincidencias con evidencia". Godoy Cruz al 2400 and Guatemala 5915 appear there too; four listings appear under "Otras opciones por confirmar".

**Automated:** `frontend/journeys/searcher.journey.ts` (first journey). The reload is tested only against a mocked API, in `frontend/e2e/search.e2e.ts` ("clarification survives reload").
