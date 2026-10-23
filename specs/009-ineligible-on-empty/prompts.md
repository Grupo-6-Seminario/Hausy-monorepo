# Show the listings you do not qualify for when nothing else is left: prompts for Claude Code

**Date**: 2026-10-23 | **Spec**: [spec.md](./spec.md)

## Step 1: clean up merged branches

```text
Branch cleanup, approved by me. Update local dev from origin/dev. Then delete, locally and on GitHub,
only these branches, whose PRs are merged (#31 to #36): eligibility-fixes,
fix/empty-and-uncertain-results, feat/eligibility-label-reasons, feat/income-proof-rule,
fix/qualification-form, feat/guarantee-rejections. They were squash-merged, so git branch -d will
call them unmerged; confirm each PR is merged with gh before using -D. Do not touch branches of other
teammates (Nick2611/testing-framework, feat/frontend-redesign, refactor/property-matching-explanations)
or main and dev. Show git branch -vv and git branch -r at the end.
```

## Step 2: build

```text
Implement specs/009-ineligible-on-empty/spec.md on a new branch from the updated dev. Read
AGENTS.md, docs/agents/workflow.md, docs/agents/backend.md, frontend/AGENTS.md, docs/agents/testing.md,
CONTEXT.md and the whole spec first.

This is a new feature: present the brief (behavior with an example, exclusions, acceptance criteria,
stopping condition, verification) and wait for my approval before coding. The API change is additive
and optional; say exactly which field you add.

Skills: codebase-design to trace how internal/buyer hides ineligible results and how property-list
renders the empty screen; ponytail; tdd and test-audit; blast-radius because the search response
changes; typescript-best-practices and badesign-skill for the folded section and cards; deslop and
code-review with at most one subagent; unslop for the report. Keep everything in this one branch so
it can be reverted with a single squash commit.

Done when every acceptance item in the spec has a test or a recorded check, go test ./..., the Postgres
tests against HAUSY_TEST_DATABASE_URI, the frontend checks and the E2E suite pass, and you show
screenshots in light, dark and mobile with the section folded and open. Use at most 4 Bedrock calls
for live screenshots. Do not commit.
```
