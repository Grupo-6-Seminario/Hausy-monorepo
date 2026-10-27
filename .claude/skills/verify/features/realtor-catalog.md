# Realtor catalog

**User sees:** `/inmobiliaria` shows "Tu catálogo", the realtor's listings with their contact counts. Each row has "Editar", which opens the dialog "Editar propiedad" with fields such as Dirección and the button "Guardar cambios". Saved edits survive a reload.

**Source:**
- `frontend/app/inmobiliaria/page.tsx`
- `frontend/app/components/agency-dashboard.tsx`
- `frontend/app/api/agency/catalog/*`
- `internal/httpapi/agency.go`
- `internal/agency`

**Drive:**
1. Sign in as the realtor and click "Ir al catálogo".
2. Click "Editar" on a row.
3. Change Dirección and click "Guardar cambios".
4. Reload and expect the new address.

The seed owns two listings: Cerviño al 4700 and the Congreso entry.

**Automated:** `frontend/journeys/realtor.journey.ts`, `frontend/e2e/agency.e2e.ts` and `frontend/app/agency-dashboard.test.tsx`.
