# Hausy / Project Angus: Kanban handoff

**Prepared:** 2026-09-26 (America/Argentina/Buenos_Aires)  
**Repository inspected:** `/Users/nicolasmendez/Documents/GitHub/proyecto-angus-back`  
**Git state at inspection:** clean `dev`, HEAD and local `origin/dev` both `0b52b2e`  
**Purpose:** source notes for a later conversation that may set up an Atlassian board. This is an analysis and proposed board seed, not an approved product roadmap or permission to create Jira issues.

## User clarification and college release scopes

Nicolás said the college releases are deliberately narrow for one subject; the product should be broader. Keep *course delivery* distinct from *product discovery and direction*. Nicolás decides the next product direction. An image supplied in the previous chat contained these scopes:

- **Release 1 — Elegibilidad temprana:** perfilado conversacional; declaración opcional por bandas; filtro sobre catálogo acotado; marcado “a confirmar”.
- **Release 2 — Comparación y lado oferta:** contrato A2A formalizado; terna con explicación y trazabilidad; vista precalificada para la inmobiliaria.

The image is a course plan, not evidence that every item is implemented or that the broader business is validated.

## What the current code does

```text
Browser conversation + optional qualification bands
  -> frontend/app/api/agent/route.ts (same-origin proxy)
  -> cmd/hausy POST /api/messages (Go)
  -> internal/intake planner (local model by default; optional Jev with fallback)
  -> Postgres candidates per branch and deterministic hard filters
  -> internal/eligibility four-state assessment and ranking
  -> optional Jev assessment of prose-only qualities when configured
  -> ranked cards streamed before the model-written reply
  -> deterministic reply template if the writer fails

Realtor browser /inmobiliaria
  -> authenticated catalog HTTP routes
  -> internal/agency.Catalog
  -> Postgres owner-scoped properties and contact-button activation counts
```

Pointers: [server wiring](../cmd/hausy/main.go), [buyer pipeline](../internal/buyer/pipeline.go), [eligibility rules](../internal/eligibility/eligibility.go), [qualification panel](../frontend/app/components/qualification-panel.tsx), [result card](../frontend/app/components/property-card.tsx), [agency contract](AGENCY_CATALOG.md).

Important details:

- Search does not require an account. The optional form takes guarantee type, income band, and whether caución was quoted. Signed-in searchers can save these answers.
- Search results distinguish `eligible`, `conditionally_eligible`, and `unknown`; a failed hard requirement produces `ineligible`, which is hidden from the main list and can contribute to a zero-results relaxation. Cards show the rule text and distinguish published attributes from model-inferred ones.
- The buyer reply discusses up to three ranked listings, but the result list can show up to ten. This is **not yet a dedicated three-property comparison**.
- The agency dashboard can add/edit/archive owned listings and shows “contactos iniciados.” A contact activation is neither a delivered inquiry nor a prequalified lead. The buyer card currently links to the external listing.
- `internal/agency.Catalog` is a typed application interface that an A2A adapter could call. No formal A2A transport or agency-side prequalified brief/view was found in `dev`.
- `docs/MATCHING_CONTRACT.md` calls the larger matching proposal parked. Its isolated experiment exists; a narrower prose-quality Jev path is wired in `cmd/hausy` when `AI_GATEWAY_API_KEY` is present. Treat these as different scopes.
- The committed data is **300 Zonaprop listings only**, 100 each from Monserrat, Congreso, and Palermo. There are 300 parsed and eligibility JSONL records; 281 quality review records (279 passed, 2 withheld). This is an intentionally bounded sample, not multi-portal coverage or evidence of CABA-wide quality.

## Verification and limits

- `go test ./...` passed on 2026-09-26.
- `cd frontend && npm test` passed: 19 files, 110 tests.
- `HAUSY_TEST_DATABASE_URI` was unset, so do not claim that the destructive disposable-Postgres integration tests were exercised in this run. No live model, browser walkthrough, or full API search was run in this review. Port 5432 was listening; ports 8080 and 8000 were not.
- No files were changed during the inspection. Recheck the branch and Git state in a new conversation; these findings describe `0b52b2e`, not future commits.

## Release interpretation

| Scope | Current assessment | Honest next check |
| --- | --- | --- |
| R1 conversational profile and optional bands | Implemented in code | Walk through a real seeded search with and without disclosure. |
| R1 bounded catalog and “a confirmar” | Bounded catalog plus explicit conditional/unknown states exist | Agree on representative acceptance cases and check the actual wording and outcomes live. |
| R2 terna with explanation and traceability | Partial: writer discusses up to three; evidence appears on cards | Nicolás/team defines what a “terna” must let a searcher compare and what provenance is required. |
| R2 formal A2A contract | Typed catalog interface is groundwork | Decide the actual exchange and authority, then implement/test that contract. Do not assume per-agency agents are settled. |
| R2 prequalified agency view | Catalog and contact counts exist | Define what the agency receives and how per-inquiry consent works, then build only the agreed slice. |

The broader December vision in the discovery repository mentions three portals, zero-results, and a consented qualified brief. That broader vision should not silently become a requirement for either narrow college release. See `/Users/nicolasmendez/Documents/GitHub/idea-validation-agents/memory/ideas/angus-buscador-decision-caba/idea.md`, `review_board.md`, and `validation_plan.md` for the dated strategic record. The review board still marks guarantee supply versus information, payer, acquisition, and A2A topology as open or contested; verify whether newer evidence exists before changing their state.

## Proposed first board structure

One board for actual team work, with **columns representing work state**:

`Inbox -> Ready -> In progress -> Review / demo -> Done`

Use `Blocked` as a visible flag with a named dependency, not as a catch-all column. A card enters **Ready** only when its owner and observable completion check are agreed. As a starting policy, try one active card per person and at most two in review; adjust based on the team's actual flow. Use a milestone/field or label for `R1`, `R2`, and `Product discovery` instead of making releases columns. The college milestones and broader product experiments can then share team capacity without sharing a definition of success.

Minimal card fields: title; type (`decision`, `experiment`, `implementation`, `verification`); milestone/track; owner; observable result and acceptance check; blocker/dependency; link to evidence, demo, or PR. Avoid filling the board from every TODO or historical commit. Record only enough Done items to show the existing foundation.

### Candidate seed cards — proposals, not approved priorities

| Candidate card | Suggested initial state | Completion check |
| --- | --- | --- |
| Walk through R1 on the seeded catalog | Ready after team agrees owner and examples | Demonstrate optional disclosure, an evidenced match, conditional/unknown status, and a zero-results case. Record what actually happened. |
| Confirm whether the three-neighborhood catalog satisfies R1 | Inbox: decision for Nicolás/course team | Written scope decision, including any required data change. |
| Define the R2 three-property comparison | Inbox: decision | One example specifies the three choices, explanation, and source evidence the viewer must see. |
| Build the agreed R2 comparison | Inbox until the preceding decision | The example works in the app and passes a review/demo. |
| Define the R2 A2A exchange and agency outcome | Inbox: decision | A typed example names sender, recipient, allowed fields, authority, consent, and agency-visible result. |
| Implement the agreed agency flow | Inbox until the preceding decision | Demonstrated exchange and prequalified view; a contact count alone does not satisfy it. |
| Reconcile dated discovery gates with outcomes | Product discovery, Inbox | Evidence is recorded or the hypothesis remains explicitly open. Do not mark a gate passed based on its planned date. |

The latest user clarification means **do not convert the college release list into the entire product roadmap**. Further product work belongs in decision or experiment cards until Nicolás chooses a direction and the supporting evidence is clear.

Kanban reference for the suggested workflow and work-in-progress practice: https://kanban.university/kanban-guide/.
