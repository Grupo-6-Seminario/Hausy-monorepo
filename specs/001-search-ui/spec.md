# Feature Specification: Natural-language property search

> Historical implementation record. The referenced `frontend/DESIGN.md` was retired on 2026-09-30. Future UI work follows [frontend instructions](../../frontend/AGENTS.md), BADESIGN, and its approved brief.
**Feature Branch**: `Nick2611/frontend-query-ui-refactor`

**Created**: 2026-09-02

**Status**: In progress

**Input**: Renew Hausy's query-first experience around persistent buyer-agent conversation and property comparison, following `frontend/DESIGN.md` and the approved cozy green palette.

## User Scenarios & Testing

### User Story 1 - Describe the right home (Priority: P1)

A person can describe hard constraints and nuanced preferences in one natural-language query, send it to the locally deployed buyer agent, and read the agent's reply beside the current property selection.

**Why this priority**: The search box is the smallest interface that can test whether nuanced intent feels better than traditional filters.

**Independent Test**: Submit a query mentioning budget, bedrooms, natural light, and noise. The request reaches the buyer agent and both sides of the turn remain visible in an accessible conversation log.

**Acceptance Scenarios**:

1. **Given** the empty search screen, **when** the person submits no text, **then** the page keeps focus in the query field and explains what is missing.
2. **Given** a nuanced query, **when** the person submits it, **then** the page sends it to the local buyer agent and shows the returned reply inline with the ranked selection.

### User Story 2 - Start from a concrete example (Priority: P2)

A person unsure how to phrase a search can load a realistic example and edit it before submitting.

**Why this priority**: Examples reveal the expected level of nuance without adding a multi-step tutorial.

**Independent Test**: Activate an example query and verify that it populates the same editable search field.

### User Story 3 - Keep the experience polished and stable (Priority: P3)

The page uses restrained state feedback, soft green surfaces, consistent rounded geometry, and an optional green vgpu luminary beneath the composer. The search flow remains usable without WebGPU.

**Why this priority**: The interface should feel calm and trustworthy throughout a high-consideration decision.

**Independent Test**: Render the page at desktop and mobile sizes and verify that the form, conversation, and results remain readable, rounded, and on palette.

### User Story 4 - Refine the search conversationally (Priority: P1)

A person can add a condition or ask a follow-up without losing earlier turns or the current shortlist.

**Why this priority**: Multi-turn refinement is the interaction being tested; replacing context on every request makes it no better than a conventional search form.

**Independent Test**: Complete an initial search, send a follow-up, and verify that the existing shortlist remains visible while Hausy works and that both completed turns remain readable afterward.

**Acceptance Scenarios**:

1. **Given** a completed search, **when** a follow-up is pending, **then** the current property selection remains visible and the UI announces an honest working state.
2. **Given** a slow request, **when** the person stops it, **then** the request is aborted and their message is restored for editing.
3. **Given** a completed response, **when** the interface settles, **then** the composer is empty and focused for the next turn.
4. **Given** a long conversation, **when** a new turn arrives, **then** the history viewport shows that turn from its beginning and retains earlier turns.
5. **Given** the property results, **when** the person follows the return link, **then** the composer receives focus for a follow-up.

### Edge Cases

- A query made only of whitespace is invalid.
- A very long query remains editable without changing the page width.
- Reduced-motion preferences remove nonessential motion.
- The local agent being unavailable produces an inline connection error and keeps the query editable.

## Requirements

### Functional Requirements

- **FR-001**: The system MUST present a labeled natural-language query field in the first viewport.
- **FR-002**: The system MUST reject empty or whitespace-only queries inline and return focus to the query field.
- **FR-003**: The system MUST forward valid queries to the local buyer-agent HTTP API without exposing the local model token to browser code.
- **FR-004**: The system MUST keep user messages and agent replies in an accessible inline conversation log.
- **FR-005**: The system MUST offer editable example queries.
- **FR-006**: The system MUST preserve the complete search flow when motion is reduced and MUST NOT depend on decorative GPU rendering.
- **FR-007**: The system MUST support keyboard submission and responsive layouts.
- **FR-008**: The system MUST preserve completed turns and the current shortlist during follow-up requests.
- **FR-009**: The system MUST let the person abort a pending request and recover the submitted message.
- **FR-010**: The system MUST identify published data and model inference separately.
- **FR-011**: The system MUST render the agent reply as a concise decision brief without exposing raw Markdown syntax.
- **FR-012**: The system MUST mark only listing cards explicitly cited by the agent's latest reply.

### Key Entities

- **Search intent**: Original query, explicit constraints, fuzzy priorities, and acceptable trade-offs.
- **Agent turn**: Session identifier, user message, returned reply, and extracted requirements.

## Success Criteria

### Measurable Outcomes

- **SC-001**: A first-time visitor can identify the primary action without scrolling.
- **SC-002**: The complete query flow is usable by keyboard at 320 CSS pixels and wider.
- **SC-003**: The local model credential remains server-side.
- **SC-004**: The primary flow passes automated tests for empty queries, proxy forwarding, conversation continuity, non-destructive loading, cancellation, and agent replies.

## Assumptions

- This slice validates UI language and interaction only; it does not prove demand or A2A value.
- The locally deployed OpenAI-compatible model is available to the Go backend during local use.
- Spanish is the initial interface language for Argentina.
