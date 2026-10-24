# Show the listings you do not qualify for when nothing else is left: spec

**Date**: 2026-10-23 | **Owner**: Martin | **Status**: approved, ready to build
**Prompts for Claude Code**: [prompts.md](./prompts.md)

When every candidate is ineligible, the empty screen says "Hay N propiedades que no te mostramos" but
not which ones or what each asks for. The searcher hits a dead end. This spec lets them open those
listings on demand, each with what it requires that they lack.

Hausy's promise stays: "Sacamos lo que no te van a aceptar antes de que lo veas". Ineligible listings
never mix with the results, appear only when there are no results, and stay folded until the searcher
asks for them.

## Behavior

- **Only when the search shows nothing.** If at least one listing is eligible, conditionally eligible
  or unknown, nothing changes: ineligible listings stay hidden.
- **Folded by default.** Below the empty-screen message and the "Si conseguís…" lines, a button reads
  "Ver la propiedad que no acepta tu situación" or "Ver las N propiedades que no aceptan tu situación".
  Opening it shows the cards; closing it hides them again.
- **Each card says what is missing.** The badge reads "No calificás", and the reason line names each
  unmet requirement in Spanish, from the catalog labels and the listing's own words, for example
  "Pide seguro de caución" or "No acepta mascotas".
- **Order:** fewest unmet requirements first, then the search's usual order. At most 10 cards.
- **The API** adds an optional, additive list of ineligible results to the search response, sent only
  when there are no visible results. Existing fields do not change.
- **Out of scope:** the model's written reply does not change; the contact button and the agency
  dashboard do not change; ineligible listings are never counted as matches.

## Acceptance

- [ ] A search where every candidate is ineligible shows the folded button with the right count; a
      search with any visible result does not show it.
- [ ] Opening it lists the listings, at most 10, ordered by fewest unmet requirements, each with "No
      calificás" and its missing requirements in Spanish.
- [ ] With guarantee "No tengo", pets "Ninguna", "departamento de 3 ambientes en Palermo hasta 1000
      dólares": the button shows 1 listing, and its card says it asks for seguro de caución.
- [ ] A search with results returns no ineligible list in the API response.
- [ ] Keyboard: the button opens and closes with Enter and Space, and focus moves to the first card.
- [ ] Revertible in one step: the whole feature is one PR merged with "Squash and merge".
