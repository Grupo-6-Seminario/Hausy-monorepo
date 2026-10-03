# Eligibility filter: prompts for Claude Code

**Date**: 2026-10-02 | **Spec**: [spec.md](./spec.md)

Each phase ends in a prompt for Claude Code. The what and why of each phase are in
[spec.md](./spec.md); this file holds the how. To run a phase, ask Claude Code:

```text
Read specs/004-eligibility-filter/prompts.md and run phase 2.
```

`AGENTS.md` makes Claude Code present behavior, acceptance criteria and a verification plan, and wait
for approval before implementing. It makes no commits, pushes or PRs until `ship` is invoked. The
prompts rely on that flow.

## Execution order

Order: 2, 1, 3, 4. Phase 2 is done (`9341292`), so phase 1 builds qualification validation on its
typed `Catalog`. Phase 4 adds the edge cases that did not come out of TDD in the earlier phases.

## Phase 1: scope

Phase 1 makes the backend the single source of eligibility facts. Today the form hardcodes them, and
the code itself notes they should be served by the backend once they change
(`frontend/app/components/qualification-panel.tsx:9`).

**In scope:**

- The high and medium priority facts in the spec: guarantee, documented income, pets, income band and
  caución quoted.
- `propietaria` stays one guarantee value (spec 1.5, decision 1, revised): no split.
- Pets as an eligibility rule (spec 1.5, decision 2).
- A new endpoint, `GET /api/eligibility/facts`, that returns the facts and their choices.
- Qualification validation on `PUT /api/me/qualification` and `POST /api/messages`, with stable error
  codes.
- The frontend form reads its choices from the endpoint.
- Pets rules derived from `listing_attributes` (`pets_allowed = no` with `provenance = stated`, and
  evidence that names an animal), with no new model calls.

**Out of scope:**

- Employment type and job tenure (low priority).
- Occupancy cap and move-in cost: deferred (spec 1.5, decisions 3 and 4).
- Regenerating or editing `data/listings.eligibility.jsonl`: it is committed model output, and
  regenerating it costs paid Jev calls.
- Error codes outside the table below (auth and agency errors, an empty message): they keep
  `{"error"}` only.

## Phase 1: endpoint contract

### GET /api/eligibility/facts

Public, no session: search needs no account. Reads `eligibility_facts` and returns only admissible
facts, ordered by priority. Response `200` with `Cache-Control: max-age=300`:

```json
{
  "facts": [
    {
      "name": "guarantee",
      "label": "Garantía",
      "priority": "high",
      "multiple": true,
      "choices": [
        {"value": "propietaria", "label": "Garantía propietaria"},
        {"value": "caucion", "label": "Seguro de caución"},
        {"value": "recibos_garante", "label": "Recibos de sueldo de un garante"},
        {"value": "none", "label": "No tengo"}
      ]
    },
    {
      "name": "income_documented",
      "label": "¿Podés comprobar tus ingresos?",
      "priority": "high",
      "multiple": false,
      "choices": [{"value": "yes", "label": "Sí"}, {"value": "no", "label": "No"}]
    },
    {
      "name": "pets",
      "label": "Mascotas",
      "priority": "high",
      "multiple": true,
      "choices": [
        {"value": "none", "label": "Ninguna"},
        {"value": "dog", "label": "Perro"},
        {"value": "cat", "label": "Gato"},
        {"value": "other", "label": "Otra"}
      ]
    }
  ]
}
```

The example shows three facts; the real response also has `income_band` and `caucion_quoted`. Labels
stay in Spanish because the searcher reads them. "Prefiero no decir" is not a choice: it means not
sending the fact.

Data changes behind the endpoint: migration `0007_eligibility_facts_v2.sql` adds `label`, `priority`,
`multiple` and `position` columns to `eligibility_facts`, seeds `income_documented` and `pets`,
and adds `recibos_garante` and `none` to the `guarantee` choices.
Choices change from a list of strings to `{value, label}` objects.

### Qualification validation

`PUT /api/me/qualification` and the `qualification` field of `POST /api/messages` validate the
qualification against the same catalog before using it:

1. Each key is a fact that exists.
2. That fact is admissible.
3. Each value is one of its choices.
4. A fact with `multiple: false` has a single value.

On failure the response carries the first error and its code; nothing is saved or searched. An empty
qualification is valid.

## Phase 1: error codes

Errors return only `{"error": "<message>"}` today, so the frontend cannot tell them apart without
reading the text. Phase 1 adds a stable `code` and, when it applies, the offending `field`. The
`error` field is unchanged, so the current frontend keeps working.

```json
{
  "error": "Elegí una de las opciones de Mascotas.",
  "code": "invalid_value",
  "field": "qualification.pets"
}
```

| HTTP | `code` | When | Message shown to the searcher |
| --- | --- | --- | --- |
| 400 | `invalid_json` | The body is not JSON, has unknown fields or exceeds 64 KiB | La solicitud no es válida. |
| 400 | `unknown_fact` | A qualification fact is not in the catalog | Ese dato no está entre las preguntas. |
| 400 | `inadmissible_fact` | The fact exists but is not admissible (age, nationality, gender) | Hausy no pide ese dato. |
| 400 | `invalid_value` | A value is not among the fact's choices | Elegí una de las opciones de {label}. |
| 400 | `too_many_values` | A single-choice fact has several values | {label} admite una sola opción. |
| 401 | `unauthenticated` | The session is missing or expired | Tenés que iniciar sesión. |
| 409 | `pending_clarification` | A search question is pending | Respondé o editá la pregunta pendiente antes de seguir. |
| 409 | `stale_clarification` | The answer is to a question that is no longer active | Esa pregunta ya no está activa. Volvé a la búsqueda. |
| 500 | `internal` | Reading the saved qualification fails | No pudimos leer tus datos. |
| 500 | `internal` | Saving the qualification fails | No pudimos guardar tus datos. |
| 500 | `internal` | Reading the catalog for `GET /api/eligibility/facts` fails | No pudimos cargar las preguntas de requisitos. |
| 500 | `internal` | Reading the catalog to validate a qualification fails | No pudimos validar tus datos. |
| 502 | `agent_unavailable` | The model did not answer | El agente local no pudo responder. |
| 503 | `no_database` | The server runs without a database, on `/api/me/qualification` | No hay base de datos para guardar tus datos. |
| 503 | `no_database` | The server runs without a database, on the facts endpoint or a search with a qualification | El servicio no tiene base de datos disponible. |

The 409 and 502 codes and the `invalid_json` and `unauthenticated` messages already exist in
`internal/httpapi`; this phase adds the `code`. The `internal` and `no_database` messages name what
failed on each path instead of "tu perfil". Validation errors use 400, not 422, to
match the agency catalog.

## Phase 1: tasks and verification

Each step ends with green tests before the next one.

1. **Migration 0007.** New `eligibility_facts` columns, `income_documented` and `pets` seeds, choices
   as objects. Postgres test with `HAUSY_TEST_DATABASE_URI`.
2. **Validation in `internal/eligibility`.** A pure function that takes the catalog and a
   qualification and returns the first typed error. One test per code in the table.
3. **Endpoint and codes.** `GET /api/eligibility/facts`, validation on both qualification endpoints,
   and `code` on every error response in the error table above. One HTTP test per error table row.
4. **Rules at load.** `load` derives a `pets subset_of` rule for each listing whose stated
   `pets_allowed` evidence names an animal as a whole word. The rule lists the pets choices the
   listing admits, read from the catalog: a refusal admits every choice but the species it names,
   or only `none` when it names none ("gatos no" admits `none, dog, other`); "sólo gatos" admits
   `none, cat`; a preference or a size limit near an animal word ("preferentemente", "mascotas
   grandes", "apto mascotas pequeñas") gives a discretionary `[none]`. Tests: each committed
   phrase, an inferred refusal, a listing that welcomes pets, a refusal whose evidence is about
   something else, "seguro obligatorio" (holds "gato"); running `load` twice leaves the same rules.
5. **Frontend.** `qualification-panel.tsx` fetches choices from the endpoint through the
   `frontend/app/api` proxy. If the endpoint fails, the form hides and search continues without a
   qualification. Vitest tests in `qualification-panel.test.tsx`.

**Done when:**

- [ ] `go test ./...` and `cd frontend && npm test` pass.
- [ ] Postgres tests pass against the disposable test database.
- [ ] On the local server, the form shows guarantee, documented income and pets as the endpoint
      returns them.
- [ ] A search with a dog declared does not show pet-refusing listings as eligible.
- [ ] Sending `age` in the qualification returns 400 `inadmissible_fact` and stores nothing.

## Phase 1: prompt

```text
Phase 1 of the Hausy eligibility filter. Read AGENTS.md, docs/agents/backend.md, docs/agents/data.md,
docs/agents/testing.md, CONTEXT.md, docs/adr/0001-eligibility-rules-as-data.md and
specs/004-eligibility-filter/spec.md (sections 1.3 to 1.5) before starting. The contract and the
error codes are in specs/004-eligibility-filter/prompts.md under
"Phase 1". Decision 2 in spec 1.5 is approved, decision 1 was revised (propietaria stays one value), and 3
and 4 are deferred.

Goal: make the backend the single source of eligibility facts and validate the declared
qualification with stable error codes.

Build:
1. Migration 0007: eligibility_facts gains label, priority (high|medium|low), multiple and position;
   choices become [{value,label}]. Seed income_documented (yes|no, single choice) and pets
   (none|dog|cat|other, multiple; none stands alone). Add recibos_garante and none to guarantee.
2. In internal/eligibility, a pure function that validates a Qualification against the catalog and
   returns the first typed error: unknown_fact, inadmissible_fact, invalid_value, too_many_values.
3. GET /api/eligibility/facts, public, admissible facts only, ordered by position,
   Cache-Control max-age=300. Validate the qualification on PUT /api/me/qualification and on the
   qualification field of POST /api/messages. Add "code" (and "field" when it applies) to every
   error response in the table, with the table's messages. Validation errors are 400.
4. In cmd/listings load, derive a pets subset_of rule (source parsed) for listings with a stated
   pets_allowed whose evidence names an animal, as task 4 above describes. No model calls.
5. qualification-panel.tsx reads its choices from the endpoint through the frontend/app/api proxy;
   on failure it hides the form and search continues without a qualification.

Out of scope: employment type, job tenure, occupancy cap, move-in cost and regenerating
data/listings.eligibility.jsonl. Ask before any paid call to Jev or Bedrock.

Done when go test ./..., the Postgres tests against HAUSY_TEST_DATABASE_URI and cd frontend && npm test
pass, and on the local server: the form shows the endpoint's choices, a search with a dog does not
mark pet-refusing listings eligible, and sending age returns 400 inadmissible_fact without storing anything.

Present the behavior, acceptance criteria and verification plan first, and wait for my approval.
Do not commit or push.
```

## Phase 2: evaluator with a Strategy per operator

A refactor of `internal/eligibility` that keeps results, except one: an unknown operator becomes
"unverifiable" instead of being evaluated as `one_of`.

```text
Phase 2 of the Hausy eligibility filter: evaluator architecture. Read AGENTS.md,
docs/agents/backend.md, docs/agents/testing.md, docs/adr/0001-eligibility-rules-as-data.md and
specs/004-eligibility-filter/spec.md (phase 2).

Goal: replace the per-operator if in outcome() in internal/eligibility/eligibility.go with a table
of functions (a Strategy made of functions, no interfaces) and type the loose strings.

Build:
1. Reason, Operator and Hardness types with constants. The JSON of Verdict and Rule does not change.
2. func comparison(op Operator) func(declared []string, rent listing.Money, r Rule) Reason, a
   switch with one_of and income_multiple (a map hashes on every rule and regressed the
   benchmark). An operator missing from the table returns unverifiable.
3. A Catalog type (facts with admissible, choices, priority, multiple) that replaces the
   admissible map[string]bool parameter of Assess and Relaxations. Migrate every caller
   (internal/buyer/pipeline.go) and delete the old signature in the same change.
4. rulesFor in internal/store/postgres validates each rule on read: an unknown operator or hardness
   is an error with context, not one more rule.

Done when: the 8 existing tests in eligibility_test.go pass without changing their expected values;
a new test covers the unknown operator (state unknown, reason unverifiable); go test ./... passes;
and BenchmarkAssess does not regress per benchstat against docs/metrics/2026-09-28/bench.txt re-run
on the same machine.

Do not add Chain of Responsibility or Specification. Present the plan and wait for my approval.
Do not commit or push.
```

## Phase 3: implementation by layers

Completes each layer without adding one, building on phases 1 and 2.

```text
Phase 3 of the Hausy eligibility filter: layers. Read AGENTS.md, docs/agents/backend.md,
docs/agents/testing.md, docs/DATA_MODEL.md and specs/004-eligibility-filter/spec.md (phase 3).

Layers and where they live (do not create new packages or services):
- Entities: internal/eligibility. DTOs for the public JSON: only in internal/httpapi.
- Repository: internal/store/postgres.
- Service: pure functions in internal/eligibility plus orchestration in internal/buyer.
- Controller: internal/httpapi.

Build:
1. Repository: Facts(ctx) returns the Catalog. Fix the N+1 in candidates() in
   internal/store/postgres/eligibility.go (it calls ByURL per listing, line 104): one query that
   returns listings, attributes and rules.
2. Domain: Validate returns a typed error (code and field). No HTTP in the domain.
3. Controller: one function in httpapi maps domain errors with errors.As to the HTTP status and code
   in the phase 1 table. The facts and error DTOs live here.
4. Logs: counts and states, never declared qualification values.

Done when: go test ./... and the Postgres tests against HAUSY_TEST_DATABASE_URI pass; an
integration test shows Candidates returns the same listings and rules as before; you ran
go run ./cmd/metrics -bench -out docs/metrics/<date> before and after and show me the packages.csv
and benchmark comparison.

Apply blast-radius because the search query changes. Present the plan and wait for my approval.
Do not commit or push.
```

## Phase 4: test cases

The full case table is in phase 4 of [spec.md](./spec.md). The prompt carries the new cases.

```text
Phase 4 of the Hausy eligibility filter: edge cases. Read docs/agents/testing.md, the test-audit
skill and specs/004-eligibility-filter/spec.md (phase 4) before writing.

Add table-driven tests in internal/eligibility/eligibility_test.go with literal expected values
(state and reason). Rent is ARS 800,000 unless stated; "3x" = income triples the rent.
1. Meets income (band 3000000-) but only has caución, and the listing requires hard propietaria
   and 3x: ineligible, guarantee not_met, the income rule in Met.
2. Propietaria declared, no band; listing with hard propietaria and 3x: unknown, missing.
3. Caución declared; listing with discretionary propietaria: conditionally_eligible, discretionary.
4. Caución declared; listing with hard propietaria and 3x, no band: ineligible (not_met wins).
5. Propietaria and caución with band 3000000-; listing with hard caución and 3x: eligible, two Met.
6. Band 3000000- and rent ARS 1,000,000 with 3x: eligible (the minimum equals the threshold).
7. Dog declared; listing refuses pets: ineligible. Pets undeclared: unknown.
8. income_documented no; listing with hard documented income: ineligible.
9. Relaxations: propietaria declared and 3 caución-only listings returns caucion/3; if one of them
   also requires 3x and the band falls short, it returns caucion/2.

Add httptest tests for each row of the phase 1 error table (specs/004-eligibility-filter/prompts.md),
and an internal/buyer test with fakes that checks the order eligible, conditionally_eligible,
unknown, and that ineligible listings are hidden and counted for zero results.

Before adding each test, name the behavior it protects. If an existing test already covers a case,
do not duplicate it. Done when go test ./... passes and you show me which cases were new and which
already existed. Do not commit or push.
```
