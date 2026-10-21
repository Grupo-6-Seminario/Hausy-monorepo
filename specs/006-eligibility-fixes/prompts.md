# Eligibility engine fixes: prompts for Claude Code

**Date**: 2026-10-21 | **Spec**: [spec.md](./spec.md)

Run the steps in order. Each waits for approval and never commits; publish with `/ship` after step 2.

## Step 1: fix the five errors

```text
Fix the five eligibility errors in specs/006-eligibility-fixes/spec.md. Read AGENTS.md,
docs/agents/workflow.md, docs/agents/backend.md, docs/agents/data.md, docs/agents/testing.md,
frontend/AGENTS.md, CONTEXT.md, docs/adr/0001-eligibility-rules-as-data.md and the whole spec first.

These are bugs that restore intended behavior: use diagnosing-bugs to reproduce each one with a
failing test before fixing it, then ponytail for the smallest fix, tdd and test-audit. Use codebase-design to trace the chat path
in internal/intake, internal/clarification and internal/buyer, and blast-radius because the change
touches eligibility semantics, the planner and the clarification flow. Apply
principle-encode-lessons-in-structure to error 3 and 4: the catalog becomes the single source, so
no qualification value stays hard-coded. Use typescript-best-practices for the frontend change.
Finish with deslop and code-review, with at most one subagent for the independent review, and
unslop for the report.

Order, one verifiable unit each:
1. Error 5 (rule validation), since the others add rules.
2. Error 1 (pets refusals from the description), reusing PublishedRules' logic.
3. Error 2 (income multiple over rent plus expensas). Ask me before changing a database schema; a
   new operator value or a field inside the rule's existing columns needs no migration.
4. Error 4 (labels from the catalog).
5. Error 3 (chat reads the catalog), including the Jev planner where it maps qualification.

Do not edit data/listings.eligibility.jsonl and make no paid model calls; derive at load time.
Out of scope: the improvements listed in the spec's opening.

Done when every acceptance item in the spec has a test or a recorded check, go test ./..., the
Postgres tests against HAUSY_TEST_DATABASE_URI and the frontend checks in docs/agents/testing.md pass.
Present the behavior, acceptance criteria and verification plan first and wait for my approval.
Do not commit or push.
```

## Step 2: verify in the app

```text
Verify specs/006-eligibility-fixes against the running app. Reload the rules with
go run ./cmd/listings load, restart go run ./cmd/hausy and check each acceptance item of the spec:
1. Find listing 59530580 (Palermo, USD 1,000) in a Palermo search in dollars with guarantee caución
   and pets dog: it must not show.
2. Find the "alquiler y expensas" income listing and show the threshold the engine used.
3. In the chat, write "busco en Palermo hasta 1.000.000, tengo un perro y seguro de caución" and
   confirm the declared qualification and the states match the same answers in the form.
4. Search with guarantee none and confirm the zero-results line labels every instrument in Spanish.
Report a table: item, how it was checked, result. Do not change code in this step.
```
