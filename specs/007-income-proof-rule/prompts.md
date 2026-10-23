# Documented income rule and the caución quote question: prompts for Claude Code

**Date**: 2026-10-22 | **Spec**: [spec.md](./spec.md)

## Step 1: build

```text
Implement specs/007-income-proof-rule/spec.md. Start a new branch from the updated origin/dev.
Read AGENTS.md, docs/agents/workflow.md, docs/agents/backend.md, docs/agents/data.md,
docs/agents/testing.md, frontend/AGENTS.md, CONTEXT.md, docs/adr/0001-eligibility-rules-as-data.md
and the whole spec first.

This changes product behavior, so present the brief (behavior with an example, exclusions, acceptance
criteria, stopping condition, verification) and wait for my approval before coding.

Skills: codebase-design to trace how load derives the pets refusals (reuse that path for the income
rule), ponytail, tdd and test-audit for each behavior, blast-radius because eligibility semantics,
the facts endpoint and the form change, typescript-best-practices and badesign-skill for the form,
domain-modeling to update CONTEXT.md if a term changes, deslop and code-review at the end with at most
one subagent for the independent review, and unslop for the report.

Order, one verifiable unit each:
1. The income_documented rule derived at load from the description. Before writing the matcher,
   list every matching clause in the current listings so I can review precision; exclude a
   guarantor's pay slips.
2. Remove caucion_quoted: facts endpoint, form, chat declarables, planner schema, saved answers,
   and drop it silently when a request still carries it. Ask me before any schema change other than
   deleting its rows.
3. Card reason line for the new rule, from the catalog label.

No paid model calls and no edits to data/listings.eligibility.jsonl. Done when every acceptance item
in the spec has a test or a recorded check, and go test ./..., the Postgres tests against
HAUSY_TEST_DATABASE_URI, the frontend checks and the E2E suite pass. Do not commit.
```

## Step 2: verify in the app

```text
Reload the rules with go run ./cmd/listings load, restart go run ./cmd/hausy and npm run dev, and
check in the running app, with screenshots:
1. A listing that asks for documented income, searched with "Ingresos comprobables: No": hidden.
2. The same search with "Sí": it shows with its other verdicts and the reason line names the rule.
3. The same search with the question unanswered: "A confirmar".
4. The form has no caución quote question.
Use at most 4 Bedrock calls. Report a table: item, how it was checked, result. Do not change code.
```
