# Contactar

**User sees:** clicking "Contactar" on a card opens the publication in a new tab and records one contact intent. The realtor then sees the listing's contact count go up in their catalog.

**Source:**
- `frontend/app/components/property-card.tsx`
- `frontend/app/api/listings/[listingId]/contact-intents/route.ts`
- `internal/httpapi/agency.go` (`POST /api/listings/{id}/contact-intents`)
- `internal/agency`

**Drive:**
1. Search Palermo with amenities, answer "Pileta".
2. On the Cerviño al 4700 card, click "Contactar" while waiting for `page.waitForEvent('popup')`.
3. Expect the popup title "Publicación 60023997", which the fake model serves.
4. Confirm the count through the API: sign in as the realtor at `POST http://127.0.0.1:8711/api/auth/sign-in`, then read `contact_count` from `GET /api/agency/catalog`.

**Automated:** `frontend/journeys/searcher.journey.ts` (first journey) and `frontend/e2e/search.e2e.ts` ("Contactar records one intent").
