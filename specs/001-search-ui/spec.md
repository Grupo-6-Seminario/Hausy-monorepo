# Feature Specification: Natural-language property search

**Feature Branch**: `feat/ui-prototype`

**Created**: 2026-09-02

**Status**: In progress

**Input**: Start the Hausy UI in a separate worktree with a modern query-first experience inspired by Startups Argentina, Curations Supply, the Cozy Green palette, taste-skill, Spec Kit, and vgpu.

## User Scenarios & Testing

### User Story 1 - Describe the right home (Priority: P1)

A person can describe hard constraints and nuanced preferences in one natural-language query, send it to the locally deployed buyer agent, and read the agent's reply in a dialog.

**Why this priority**: The search box is the smallest interface that can test whether nuanced intent feels better than traditional filters.

**Independent Test**: Submit a query mentioning budget, bedrooms, natural light, and noise. The request reaches the buyer agent and its reply appears in an accessible dialog.

**Acceptance Scenarios**:

1. **Given** the empty search screen, **when** the person submits no text, **then** the page keeps focus in the query field and explains what is missing.
2. **Given** a nuanced query, **when** the person submits it, **then** the page sends it to the local buyer agent and shows the returned reply in a dialog.

### User Story 2 - Start from a concrete example (Priority: P2)

A person unsure how to phrase a search can load a realistic example and edit it before submitting.

**Why this priority**: Examples reveal the expected level of nuance without adding a multi-step tutorial.

**Independent Test**: Activate an example query and verify that it populates the same editable search field.

### User Story 3 - Keep the experience polished without blocking access (Priority: P3)

The page uses a subtle vgpu visual layer when WebGPU is available and remains fully usable when it is not.

**Why this priority**: Visual novelty is secondary to completing the search task.

**Independent Test**: Render with no WebGPU support and verify that the form, content, and fallback background still work.

### Edge Cases

- A query made only of whitespace is invalid.
- A very long query remains editable without changing the page width.
- WebGPU initialization failure never hides or disables the form.
- Reduced-motion preferences remove nonessential motion.
- The local agent being unavailable produces an inline connection error and keeps the query editable.

## Requirements

### Functional Requirements

- **FR-001**: The system MUST present a labeled natural-language query field in the first viewport.
- **FR-002**: The system MUST reject empty or whitespace-only queries inline and return focus to the query field.
- **FR-003**: The system MUST forward valid queries to the local buyer-agent HTTP API without exposing the local model token to browser code.
- **FR-004**: The system MUST show the agent's returned reply in an accessible dialog.
- **FR-005**: The system MUST offer editable example queries.
- **FR-006**: The system MUST preserve the complete search flow when WebGPU is unavailable or motion is reduced.
- **FR-007**: The system MUST support keyboard submission and responsive layouts.

### Key Entities

- **Search intent**: Original query, explicit constraints, fuzzy priorities, and acceptable trade-offs.
- **Agent turn**: Session identifier, user message, returned reply, and extracted requirements.

## Success Criteria

### Measurable Outcomes

- **SC-001**: A first-time visitor can identify the primary action without scrolling.
- **SC-002**: The complete query flow is usable by keyboard at 320 CSS pixels and wider.
- **SC-003**: The local model credential remains server-side.
- **SC-004**: The primary flow passes automated tests for empty queries, proxy forwarding, and agent replies.

## Assumptions

- This slice validates UI language and interaction only; it does not prove demand or A2A value.
- The locally deployed OpenAI-compatible model is available to the Go backend during local use.
- Spanish is the initial interface language for Argentina.
