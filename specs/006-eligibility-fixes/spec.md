# Eligibility engine fixes: spec

**Date**: 2026-10-21 | **Owner**: Martin | **Status**: approved, ready to build
**Prompts for Claude Code**: [prompts.md](./prompts.md)

A review of the eligibility engine on `dev` (`7c70120`) found that `Assess` decides correctly with the
rules it receives. Five errors sit around it: rules that are missing or incomplete, a chat that drops
half of the qualification, and labels that drift from the catalog. Errors 1 and 2 show "Calificás" to
a searcher who does not qualify, the worst outcome the filter can produce.

Improvements from the same review (documented income rules, finer income bands, explicit guarantee
rejections, a fresh extraction eval, the no-guarantee versus discretionary decision) are out of scope.

## Errors

### 1. A "sin mascota" listing is eligible for a dog owner

Pets rules come only from the parser's `pets_allowed` attribute (`eligibility.PublishedRules`). In
listing `59530580` ("requisitos seguro de caución [...] sin mascota") the parser left it unset, so the
listing has no pets rule and a dog owner who has caución sees it as eligible.

**Fix:** when a listing has no stated `pets_allowed` attribute that names an animal, `load` also reads
explicit refusals in the description: "sin mascota(s)", "no se aceptan/permiten mascotas", "no acepta(n)
mascotas", "no (es) apto mascotas", "prohibido mascotas", and the same with "perros", "gatos" or
"animales". The matched clause goes through the same logic as a stated attribute with value `no`, so
species refusals, "sólo gatos" and size limits behave as they already do. The clause is the rule's
evidence. No model calls; `data/listings.eligibility.jsonl` is not edited.

### 2. An income rule ignores expensas

One of the four income rules says "recibos de sueldo que tripliquen el valor del alquiler y expensas",
but `incomeMultiple` multiplies the rent only, so a searcher below the real threshold can be eligible.

**Fix:** a rule records whether the multiple applies to rent plus expensas. When it does, the
threshold is multiple × (rent + expensas). If the listing publishes no expensas, or they are not in
ARS, the rule is unverifiable. At load, an income rule whose evidence mentions "expensas" gets the
flag; `Extract` sets it the same way for future runs. Rent-only rules behave as today.

### 3. The chat drops pets, "no tengo garantía" and a guarantor's pay slips

The form reads the catalog, but the chat path hard-codes the old values: the Qwen planner's schema
(`internal/intake/qwen.go:184`), the clarification prompt, fact list and effect validation
(`internal/clarification/clarification.go:108`, `:140`, `:187`), the guarantee question
(`internal/clarification/judge.go:162`) and the cue check (`internal/buyer/clarification.go:404`).
A searcher who writes "tengo un perro" or "no tengo garantía" gets "A confirmar" instead of
"No califica".

**Fix:** every chat path takes its facts and choices from the same catalog the form uses: the planner
schema and its sanitizer, the clarification prompt, the allowed facts and values, and the guarantee
question's choices (propietaria, caución, recibos de un garante, no tengo). Volunteered qualification
is validated with `eligibility.ValidateQualification`. The Jev planner follows the same rule where it
maps qualification.

### 4. "Si conseguís recibos_garante" in the zero-results line

The guarantee labels are written by hand in `frontend/app/components/property-list.tsx:25` and
`internal/buyer/writer.go:60` and lack `recibos_garante`.

**Fix:** both take the label from the catalog choices. The writer receives the catalog; the frontend
uses the facts endpoint it already calls for the form.

### 5. A rule without values hides the listing from everyone

`one_of` and `subset_of` with no values return `not_met`, so the listing becomes ineligible for every
searcher, and `Rule.Validate` accepts such a rule. It would happen if `load` ran with a catalog that
lacks the pets choices.

**Fix:** `Rule.Validate` rejects a rule with no values, and `LoadEligibility` fails loudly instead of
saving it.

## Acceptance

- [ ] Listing `59530580` has a pets rule after `load`; a dog owner with caución sees it hidden, and a
      searcher with no pets sees it eligible.
- [ ] The "alquiler y expensas" listing is not eligible for a band whose minimum covers 3 × rent but not
      3 × (rent + expensas).
- [ ] In the chat, "tengo un perro", "no tengo garantía" and "tengo garante con recibo de sueldo" end
      in the same states as the same answers in the form.
- [ ] The zero-results line and the model's reply say "recibos de sueldo de un garante".
- [ ] Loading a rule with no values fails with an error that names the listing.
- [ ] The 14 Phase 4 edge cases still pass with their expected values unchanged.
