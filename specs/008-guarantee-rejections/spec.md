# Explicit guarantee rejections: spec

**Date**: 2026-10-23 | **Owner**: Martin | **Status**: approved, ready to build
**Prompts for Claude Code**: [prompts.md](./prompts.md)

A listing that says which guarantee it does **not** accept ("No se aceptan seguros de caución") gets no
guarantee rule today, so a searcher who only has that guarantee sees it as "A confirmar" instead of
"No califica". This spec turns explicit rejections into a rule.

**Scope today:** one of the 300 committed listings states a rejection: 59787969 (id 235, Monserrat,
ARS 780,000), "(No se aceptan seguros de caución)", with no other guarantee rule. Listings that say
"sólo garantía propietaria" are already covered: they accept only propietaria. The value of this
change grows with the CABA-wide catalog.

## Behavior

- At load, a listing whose description rejects a guarantee instrument by name gets a rule saying which
  instruments it rejects, hard, source `parsed`, with the clause as evidence. Phrases: "no se
  acepta(n)", "no acepta(mos)", "no se admite(n)", "no trabajamos con", followed by an
  instrument's words (the same instrument patterns `eligibility.Instruments` uses). Deterministic; no
  model calls; `data/listings.eligibility.jsonl` is not edited.
- "Sin" is not a rejection cue (decided 2026-10-23): "sin garantía propietaria" says the listing does
  not require one, not that it refuses it, like "No requerimos garantía inmobiliaria".
- A searcher whose declared guarantees are **all** rejected does not meet it: **No califica**. "No
  tengo garantía" is not rejected by a rule that names an instrument.
- A searcher with at least one guarantee the listing does not reject has not shown that the listing
  **accepts** it. The rejection rule alone must never make a listing eligible: the verdict comes from
  the other rules, and with none it stays **A confirmar**.
- A listing that both accepts and rejects instruments keeps both rules; the evaluator combines them as
  it combines any two rules.
- The zero-results line ("Si conseguís…, vuelven N") may suggest an instrument the listing does not
  reject only if another rule accepts it.
- The card's reason line says which guarantee the listing does not accept, with the catalog label.

## Acceptance

- [ ] After `load`, listing 59787969 carries a rule rejecting caución, with its clause as evidence.
- [ ] Caución only: that listing is hidden. Propietaria: it shows "A confirmar", never "Calificás".
      Caución and propietaria: "A confirmar".
- [ ] A listing with an acceptance rule and a rejection rule gives the combined verdict (unit tests
      with literal expectations).
- [ ] Every clause the matcher finds is listed once for a human to review before the change ships.
- [ ] The Phase 4 edge cases and the spec 006 and 007 acceptance tests still pass.
