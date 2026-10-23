# Documented income rule and the caución quote question: spec

**Date**: 2026-10-22 | **Owner**: Martin | **Status**: approved, ready to build
**Prompts for Claude Code**: [prompts.md](./prompts.md)

The qualification form asks two questions whose answers change nothing: no rule reads
`income_documented` or `caucion_quoted`. This spec gives the first one a rule and removes the second.

## 1. Documented income becomes a rule

37 of the 279 searchable listings (13%) ask for income the tenant can prove: "ingresos comprobables",
"demostrables", "justificables", "en blanco", or the tenant's own "recibos de sueldo" (counted on
2026-10-22 with a keyword search over descriptions).

**Behavior:** at load, a listing whose description asks for proof of the tenant's income gets the rule
`income_documented one_of [yes]`, hard, source `parsed`, with the matching clause as evidence.

- A searcher who declared "Sí" meets it; "No" makes the listing ineligible; no answer leaves it
  unknown, as every undeclared fact does.
- A guarantor's pay slips are a guarantee, not proof of the tenant's income: "garante con recibo de
  sueldo" and similar clauses do not create this rule.
- Derived deterministically at load, like the pets refusals; no model calls, and
  `data/listings.eligibility.jsonl` is not edited.
- The result card's reason line names it in Spanish. The catalog label is a question ("¿Podés comprobar
  tus ingresos?"), so the line reuses the situation bar's phrase: "Cumplís lo que pide: …, ingresos
  comprobables" (approved 2026-10-22).

**Reviewed scope (2026-10-22):** the hand review in [clauses.md](./clauses.md) settled on 65 listings, 57
of them searchable. The keyword count above missed the noun forms "demostración", "justificación" and
"comprobante de ingresos"; the matcher found 21 more listings with them, and Martin approved them.

**Expected effect:** about 57 searchable listings gain a requirement. A searcher who leaves the question blank sees
them as "A confirmar" ("Publica requisitos · completá tus datos") where some showed "Calificás" before.
That is the intended rule: an undeclared fact is never eligible.

**Known limitation: alternatives across facts.** Some listings accept proof of income *or* something else:
"ingresos comprobables o seguro de caución" (59980152) or "acreditar ingresos/estudios" (55770968,
55360032). A rule reads one fact, so `income_documented one_of [yes]` would make these listings
ineligible for a searcher who meets the other alternative. They get no income rule. This is the first
real case for combined rules, where one requirement is met by any of several facts.

## 2. The caución quote question is removed

No listing requires an existing caución quote, so the answer cannot change a verdict. The form stops
asking it, and the chat stops recording it.

- The fact leaves the facts endpoint, the form, the chat's declarable facts and the planner schema.
- Saved `caucion_quoted` answers are deleted from `user_qualifications`.
- A request that still carries `caucion_quoted`, from an old saved profile or an open browser tab,
  must not fail the search; the value is dropped.
- It can come back when it has a use, for example in the brief sent to an agency.

This reverses one of the three profile questions chosen in `specs/002-eligibility-first-search`
(decision Q19); Martin approved it on 2026-10-22.

## Acceptance

- [ ] After `load`, the reviewed listings (57 searchable, see [clauses.md](./clauses.md)) carry the
      `income_documented` rule with their own clause as evidence, and none comes from a guarantor's
      pay slips.
- [ ] Every matched clause is listed once for a human to review before the change ships.
- [ ] With "Ingresos comprobables: No", those listings are hidden; with "Sí" they keep their other
      verdicts; unanswered, they show "A confirmar".
- [ ] The form no longer shows the caución quote question; `GET /api/eligibility/facts` omits it.
- [ ] A search whose qualification still includes `caucion_quoted` succeeds.
- [ ] The Phase 4 edge cases and the spec 006 acceptance tests still pass.
