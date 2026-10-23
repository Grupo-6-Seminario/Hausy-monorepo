# Explicit guarantee rejections: prompts for Claude Code

**Date**: 2026-10-23 | **Spec**: [spec.md](./spec.md)

## Step 1: build

```text
Implement specs/008-guarantee-rejections/spec.md. Start a new branch from the updated origin/dev.
Read AGENTS.md, docs/agents/workflow.md, docs/agents/backend.md, docs/agents/data.md,
docs/agents/testing.md, CONTEXT.md, docs/adr/0001-eligibility-rules-as-data.md and the whole spec.

This changes eligibility semantics, so present the brief (behavior with an example, exclusions,
acceptance criteria, stopping condition, verification) and wait for my approval before coding.
Before writing the matcher, list every rejection clause it would match in the current listings so I
can review precision.

Design constraint: a rejection rule must never be the reason a listing is eligible. If that needs a
new operator or a new reason in the evaluator, propose it in the brief with the verdict table for
caución only, propietaria only, both, and "none". Reuse eligibility.Instruments' patterns for the
instrument words and the path load already uses for pets refusals and documented income.

Skills: codebase-design, ponytail, tdd and test-audit, blast-radius (eligibility semantics and
relaxations change), domain-modeling if CONTEXT.md needs the term, deslop and code-review with at most
one subagent, and unslop for the report. No paid model calls; do not edit
data/listings.eligibility.jsonl. Done when every acceptance item in the spec has a test or a recorded
check, and go test ./..., the Postgres tests against HAUSY_TEST_DATABASE_URI, the frontend checks and
the E2E suite pass. Do not commit.
```

## Step 2: verify in the app

```text
Reload with go run ./cmd/listings load, restart go run ./cmd/hausy and npm run dev, and search
"departamento en alquiler en Monserrat entre 770.000 y 790.000 pesos" three times: guarantee caución
only (listing 59787969 hidden), propietaria only ("A confirmar"), and both ("A confirmar"). Use at most
3 Bedrock calls. Report a table with screenshots. Do not change code.
```
