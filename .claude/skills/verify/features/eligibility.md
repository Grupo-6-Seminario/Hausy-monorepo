# Qualification and eligibility

**User sees:**
- A qualification panel: chip groups for Garantía, ¿Podés comprobar tus ingresos? and Mascotas, a select for Ingresos mensuales, and "Usar estos datos".
- The answers as a "Tu situación" summary with an "Editar" button.
- Cards marked "Calificás" with "Cumplís lo que pide", "Calificás con condiciones", or "A confirmar".
- With no shown results: "Ninguna coincide con tu situación" and the relaxation line "Si conseguís seguro de caución, vuelven N propiedades". A button, "Ver las N propiedades que no aceptan tu situación", opens cards marked "No calificás".
- For a signed-in searcher, answers are saved and reload with the page.

**Source:**
- `frontend/app/components/qualification-panel.tsx`, `property-card.tsx`
- `internal/eligibility` (`Assess`, `Relaxations`)
- `internal/buyer/pipeline.go`
- `internal/httpapi/handler.go` (`/api/me/qualification`, `/api/eligibility/facts`)
- The evaluator's promises are checked formally by the `eligibility-verdicts` skill.

**Drive:** `scripts/journey.mjs` covers this feature end to end. For "A confirmar", skip the declaration and search Palermo.

**Automated:** `frontend/journeys/searcher.journey.ts` (second journey) and `frontend/e2e/search.e2e.ts` (eligibility sections, empty result, refused listings).
