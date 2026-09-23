# Implementation plan: eligibility-first search with typed intake

**Branch**: `Nick2611/jev-integration-implementation` | **Date**: 2026-09-23
**Inputs**: grilling session 2026-09-23 (Q1–Q24), [CONTEXT.md](../../CONTEXT.md),
[ADR 0001](../../docs/adr/0001-eligibility-rules-as-data.md), intake spike
([experiments/intake](../../experiments/intake/README.md)).

## Summary

Replace the local model's tool loop with a typed pipeline. A **planner** turns the conversation into a
plan: branches, requirements and sort. SQL returns every hard-filter match per branch. A pure Go
evaluator labels each listing with its **eligibility** for the searcher's **qualification**. Results
are ordered by eligibility section, then by the user's sort (default: requirement fit), and the local
model writes one reply explaining why each property is a good match. Jev is the planner behind a switch;
Qwen, in a single structured call, is the default and the 6 s fallback.

## Decisions this plan implements

| # | Decision |
| --- | --- |
| Q1, Q24 | `HAUSY_PLANNER=jev\|qwen` (default `qwen`) selects only the planner. Everyone gets the new pipeline; the tool loop is retired. |
| Q7 | Jev planner has a 6 s budget; on timeout or error the turn falls back to the Qwen planner. |
| Q6 | The local model only (a) fills the plan in the Qwen planner and (b) writes the reply. No tools. |
| Q8 | On an `ask_about_listing` turn the plan is frozen; the reply answers from that listing's record. |
| Q9 | "Olvidate de X" means exclude X. |
| Q10 | Unspecified operation defaults to `alquiler`, shown as an editable chip. |
| Q11, Q12 | A plan has one or more **branches**. A requirement applies to every neighborhood unless tied to one. |
| Q13, Q17, Q22 | Order: `eligible` → `conditionally_eligible` → `unknown` (`ineligible` hidden). Within a section: the user's explicit sort, else requirement fit, then URL. Never price by default. One global list across branches; `#N` assigned after ordering. |
| Q15 | The reply explains why each shown property is a good match: eligibility with the named condition, requirement matches with evidence, and the zero-results line. |
| Q18, ADR 0001 | Eligibility requirements are extracted at load time and stored as data rows; evaluation is a pure Go function. |
| Q19, Q21 | Qualification comes from a skippable three-question form (instruments, income band, caución quoted). Anonymous: session only. Logged in: saved to the account. |
| Q20 | Frontend shows sections in eligibility order. |
| Q23 | Rules may only reference facts marked admissible; protected characteristics are not admissible. |

## Modules and seams

| Module | Interface | Adapters / notes |
| --- | --- | --- |
| `internal/eligibility` (new) | `Assess(q Qualification, rent listing.Money, rules []Rule) Verdict` · `Relaxations(q, []Candidate) []Relaxation` | Pure; no I/O. `Qualification` and `Rule` are data (fact names are strings), per §8. |
| `internal/jev` (new, extracted) | `Evaluate(ctx, state, questions) (answers, error)` | Moves the Gateway transport out of `internal/matching/jev` and the spike: bounded retry with backoff on 429/5xx, error type + `X-Vercel-Id`, no messages logged. Three callers (planner, extraction, matching), so a real seam. |
| `internal/intake` (new) | `Planner.Plan(ctx, turns []string, previous Plan) (Plan, error)` | Two adapters: `Jev` (port of spike v2) and `Qwen` (one JSON call). `Plan{Intent, Branches []search.Query, Sort}`; a branch **is** a `search.Query`. Fallback wrapper with a 6 s budget. |
| `search.Repository` | + `Candidates(ctx, Query) ([]eligibility.Candidate, error)` | Postgres: every hard-filter match with its rules, uncapped. `ponytail:` cap at 500 with `complete=false` if the inventory outgrows it. |
| `internal/buyer` | `HandleMessage(ctx, sessionID, message, Qualification) (*TurnResponse, error)` | Turn = plan → candidates per branch → assess → order → packet → writer. `TurnResponse.Listings[].Eligibility`, `+Relaxations`. |
| `cmd/listings` | + `eligibility` subcommand; `load` also reads `data/listings.eligibility.jsonl` | Extraction output committed, like the parse output. |
| `internal/httpapi` | `POST /api/messages` accepts `qualification`; `GET/PUT /api/me/qualification` | Account storage in `user_qualifications (user_id, fact, value)`. |
| frontend | qualification form · eligibility sections · named conditions | `--signal-conditional` finally consumed. `#N` parsing in `search-experience.tsx` unchanged. |

Parked, not wired: `internal/matching` (preference assessment by Jev) stays in the repo behind nothing
until intake ships; requirement fit v1 uses parsed attributes only.

## Slices (TDD, in order; each ends green and shippable)

1. **Eligibility evaluator.** `internal/eligibility`. Tests are worked examples written from the two real
   REQUISITOS blocks in the snapshot: propietaria accepted and searcher has it → `eligible`; searcher
   only has caución → `ineligible`; "garantía CABA o caución (ver cuáles permite)" → `conditionally_eligible`
   with the condition named; no rules → `unknown`; income band straddling 3× rent → `conditionally_eligible`;
   non-admissible fact → rule ignored and reported. Plus `Relaxations`: "si conseguís caución, vuelven N".
2. **Shared Jev client.** Extract `internal/jev` from `internal/matching/jev` and the spike. Move the existing
   adapter tests (error type, request ID, transport cause, cancellation); add retry-then-succeed and
   no-retry-on-400/403. `internal/matching/jev` becomes a thin caller.
3. **Extraction.** `cmd/listings eligibility`: deterministic spotting (instrument names, "N veces", "N meses")
   + Jev per spotted instrument (`accepted | rejected | not_stated`, `hard | discretionary`). Resumable
   JSONL, like `parse`. Hand-label 20 listings first, then run all 300 and report agreement. Commit the JSONL.
4. **Store.** Migration `0004`: `eligibility_facts (name, admissible, values)` seeded;
   `listing_eligibility_rules`; `user_qualifications`. `load` ingests the eligibility JSONL. `Candidates()`.
   Tests against Postgres, as `search_test.go` does.
5. **Planner.** `internal/intake` with both adapters and the fallback. Port the spike's extraction and v2
   questions; add branch scoping (one gate question, per-neighborhood questions only when it's true),
   sort, qualification volunteered in chat, default operation, freeze on `ask_about_listing`. Tests: fake
   Gateway server + fake `llm.Client`. Add Belgrano/Monserrat, sort and qualification cases to
   `experiments/intake/cases.json` and re-run both planners.
6. **Turn pipeline.** New `buyer` turn behind `HandleMessage`, with a fake planner, fake repository and fake
   writer. Asserts: section order, within-section sort, stable `#N`, empty branch reported,
   relaxations, frozen plan on `ask_about_listing`.
7. **Writer.** One local-model call from the packet; deterministic template on failure. Test: fake client
   error → template reply that still names each `#N`.
8. **Wiring + HTTP.** `cmd/hausy` reads `HAUSY_PLANNER`; `httpapi` passes qualification; account endpoints.
9. **Frontend.** Qualification form (skippable, prefilled when logged in); sections with named conditions;
   relaxation line. Vitest at the component seams already used (`property-list.test.tsx`).
10. **Retire the tool loop.** Delete `handleWithInventory`'s runner path, `search.Toolset` and
    `internal/tools`. As of 2026-09-23 they have no other importers. Update AGENTS.md's repo table and `docs/DATA_MODEL.md`.

Skipped, add when needed: UI sort control (users state sort in the chat for now), private agency rules
(schema ready; legal review first), declared/observed rule sources beyond `parsed`, preference matching
via Jev, cost telemetry.

## Status — 2026-09-23

All 10 slices are implemented behind tests at the seams above. `go test ./...` passes, including the
Postgres-backed tests against `hausy_test`, and the frontend passes `vitest`, `tsc` and `oxlint`.
Live checks:

- **Extraction** over all 300 listings: 20/20 accepted instruments and 10/10 hardness against the hand
  labels, after one diagnosed pattern fix (see `experiments/eligibility`; that set is no longer held out).
- **End-to-end** (`HAUSY_PLANNER=jev`, dev DB, local writer): one turn took 26 s; eligible listings came
  first and 7 caución-only listings became "si conseguís caución, vuelven 7". The writer's prose made
  wrong comparative claims ("la más económica"); the ranking and cards were correct.
- **Planners** (`experiments/intake`, 23 cases): Jev 15/23 exact (15/16 when Gateway answered; 7 Gateway
  errors), Qwen single call 17/23 with 0 errors. See `experiments/intake/README.md`.

Deferred on purpose: the Belgrano/Monserrat branch case depends on Jev's per-neighborhood answers
and needs more labeled cases before trusting it.

## Risks

- **Gateway limits:** 26 retries in 26 turns in the spike. The fallback bounds the damage; measure the
  fallback rate in slice 8 and look into Gateway rate-limit tiers.
- **Extraction quality is unmeasured.** Slice 3 labels by hand before trusting it; `unknown` is the safe
  default.
- **The Qwen planner's single-call accuracy is unmeasured.** The spike measured Qwen's tool loop, not a
  single call; slice 5 re-runs the intake cases against it.
