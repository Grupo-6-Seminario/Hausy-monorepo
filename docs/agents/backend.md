# Backend conventions

For Go work, read [the workflow](workflow.md), then trace the HTTP or command entry point through its owning domain and persistence/provider boundaries. Consult [the glossary](../../CONTEXT.md), [data model](../DATA_MODEL.md), and the applicable ADR or contract.

## Implementation

Follow surrounding Go style and use `gofmt`. Keep domain logic independent of HTTP and provider-specific types. Adapters parse external data into domain values. Propagate `context.Context` cancellation and deadlines through I/O. Handle errors explicitly with useful context and preserve error identity where callers rely on it.

Prefer existing packages and the standard library. Introduce an interface for an actual substitution or meaningful boundary, not a hypothetical future implementation. Preserve the existing vendor-neutral LLM, account, and catalog boundaries when changing adapters. Favor pure deterministic decisions where a normal function or query suffices.

Use concise comments above functions or key logic where intent or a constraint helps review. Keep transport validation at trust boundaries; avoid redundant checks on already-validated internal values. Do not simplify away authorization, data integrity, or recoverable failures.

## Contracts and state

Ask before changing public contracts, persisted schemas, or search semantics. Preserve owner-scoped catalog access, contact-intent idempotency, and content-fingerprinted quality decisions. Read [agency contracts](../AGENCY_CATALOG.md) for agency work and [matching contracts](../MATCHING_CONTRACT.md) for search work.

Commands that may be retried should converge safely after interruption. Verify relevant migrations and data behavior against a designated disposable database. Never use the development dataset as a test cleanup target.

## Diagnostics and streaming

Use the existing structured logging approach and request identifiers. Keep provider/database failure details sanitized; avoid logging secrets, raw user content, or private qualifications. Preserve streaming flush behavior through response-writer wrappers. Report logical stream outcomes separately from HTTP status after headers have been sent.

Logging work does not authorize adding metrics, dashboards, or alerts. Define and verify the requested diagnostic behavior at the actual boundary.

## Completion

Follow [testing and review](testing.md). Run focused package checks during development and `go test ./...` before completing Go changes. A passing command with skipped database or live-provider tests is not proof of those behaviors; report and run the relevant approved checks when acceptance depends on them.
