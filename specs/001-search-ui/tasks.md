# Tasks: Natural-language property search

> Historical implementation record. The referenced `frontend/DESIGN.md` was retired on 2026-09-30. Future UI work follows [frontend instructions](../../frontend/AGENTS.md), BADESIGN, and its approved brief.
## Setup

- [x] T001 Create the separate `feat/ui-prototype` worktree.
- [x] T002 Initialize the `frontend/` Vinext project and install test tooling.
- [x] T003 Record the Spec Kit-style specification and plan.

## User Story 1 - Describe the right home

- [x] T004 [US1] Write failing form-seam tests in `frontend/app/search-experience.test.tsx`.
- [x] T005 [US1] Implement the query form and deterministic interpretation in `frontend/app/components/search-experience.tsx`.
- [x] T006 [US1] Compose the first-viewport route in `frontend/app/page.tsx`.

## User Story 2 - Start from a concrete example

- [x] T007 [US2] Add editable example prompts to the search surface.

## User Story 3 - Stable visual refinement

- [x] T008 [US3] Add restrained loading and focus feedback with reduced-motion support.
- [x] T009 [US3] Register the primary search journey as an imperative WebMCP tool.

## Validation

- [x] T010 Run unit tests, production build, lint, and Go tests.
- [x] T011 Run the taste-skill pre-flight audit and inspect generated visual assets.

## User Story 4 - Reach the local buyer agent

- [x] T012 [US4] Add a tested Go `POST /api/messages` boundary for the buyer agent.
- [x] T013 [US4] Add a tested same-origin frontend proxy to the local Hausy API.
- [x] T014 [US4] Replace the deterministic result with the real agent reply in an accessible dialog.
- [x] T015 [US4] Document local model, API, and frontend startup configuration.

## User Story 5 - Renew the conversational search experience

- [x] T016 [US5] Add failing public-seam tests for persistent turns, stable follow-up results, and request cancellation.
- [x] T017 [US5] Replace the landing/result handoff with a responsive conversation-and-selection workspace.
- [x] T018 [US5] Restyle the query, messages, loading states, empty state, and listing evidence using only the `DESIGN.md` tokens.
- [x] T019 [US5] Restore the approved cozy green palette and remove the unstable decorative GPU prompt artifact.
- [x] T020 [US5] Render agent replies as semantic, concise decision briefs.
- [x] T021 [US5] Link explicit rank citations in the agent reply to the corresponding property cards.
- [x] T022 [US5] Expand the conversation column, soften all component geometry, and add the provisional Hausy logo.
