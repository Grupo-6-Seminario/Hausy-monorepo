# Search conversation and results

**User sees:** on `/`, the prompt "¿Qué estás buscando?". Sending a message moves the conversation into a side panel. That panel shows "Tu búsqueda" chips, the searcher's turns and Hausy's reply. Results appear as property cards under "Selección actual". After a first search, the prompt becomes "Sumá a tu búsqueda", and the header gains "Ver selección (N)".

**Source:**
- `frontend/app/components/search-experience.tsx`, `property-list.tsx`, `property-card.tsx`, `agent-reply.tsx`
- `frontend/app/api/agent/route.ts`
- `internal/httpapi/handler.go` (`/api/messages`)
- `internal/buyer/pipeline.go`

**Drive:**
1. `hydrated(page)`.
2. `search(page, /¿Qué estás buscando\?/, 'Busco alquiler en Palermo')`.
3. Expect Palermo cards, and Hausy's reply "Estas son las propiedades del catálogo de prueba que cumplen lo que pediste."

**Automated:** mocked browser tests in `frontend/e2e/search.e2e.ts` and component tests in `frontend/app/search-experience.test.tsx`.
