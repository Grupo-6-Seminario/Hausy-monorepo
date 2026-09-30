# Testing and completion review

## Choose proof

For behavior changes, use TDD: observe a meaningful failure before implementing its fix. Feature test boundaries are approved with the brief; established boundaries may be reused for restoring bugs. Test observable behavior with independent literal expectations, worked examples, or an approved contract.

Use domain or integration tests for backend rules, component/API tests for frontend behavior, and E2E for user journeys. Choose the strongest practical boundary for each risk. Mock remote or unavailable services when necessary, while keeping the actual subject under test. Fixture-backed UI tests do not validate listing accuracy or live model behavior.

Apply `test-audit` whenever writing, changing, reviewing, or pruning tests. Identify the protected behavior, credible regression, coverage gap, and any test-only production seam before adding a test. After long or multi-phase work, audit the affected test area for duplication, weak assertions, and implementation coupling. Preserve meaningful regressions; uncertainty is not a reason to delete a test.

## Run checks

Use current manifests and [SETUP](../SETUP.md) to verify commands. These are the current baseline:

| Change | Checks |
| --- | --- |
| Go | Focused package checks during work; `go test ./...` before completion; `gofmt` on changed Go files |
| Postgres behavior | Set `HAUSY_TEST_DATABASE_URI` to a designated disposable database ending in `_test`; run relevant store integration tests |
| Frontend | Focused tests, then `npm test`, `npm run lint`, `npx tsc --noEmit --incremental false`, and `npm run build` in `frontend/` |
| User journey or visible UI | Relevant `npm run test:ui`, running-browser inspection, screenshots, applicable keyboard/reduced-motion/error checks |
| GPU/shader behavior | `npm run test:gpu` plus actual UI fallback inspection |
| Infrastructure | Local formatting and validation; authenticated plan only after authorization |
| Agent documents or setup | `python3 scripts/check-agent-instructions.py`; `python3 scripts/verify-agent-setup.py` in a disposable checkout; independent behavioral check after substantial workflow changes |
| Every changed artifact | `git diff --check` and final diff inspection |

Database tests truncate their target. A `_test` suffix is required, but also confirm the target is disposable. Skipped tests need explicit reporting. Run only checks relevant to changed behavior, plus the domain's completion baseline. Repeat or broaden checks only for new changes, failures, or unresolved risks.

## Review

Apply `deslop` to the task diff against its actual base, normally the merge-base with `dev`, including uncommitted changes. Preserve behavior and useful short comments.

Automatically apply the normal Matt Pocock `code-review` after substantial code changes, including features, shared logic, migrations, and multi-phase fixes. This is a local review, not the paid Claude review service. Use one independent reviewer for one axis and the parent for the other, keeping Standards and Spec findings separate. A bounded trivial edit needs diff inspection without an obligatory independent review.

Use `blast-radius` for shared contracts, persistence, authentication, or search semantics; prove the relevant downstream behavior. `interrogate` and `no-comments` remain user invoked. Comments that explain purpose, constraints, or a decision are welcome.

Fix verified in-scope findings and rerun affected checks. Report unrelated findings. Ask if a correction would change approved product behavior or criteria. A finding's severity or origin does not justify accepting it without evidence.

## Done predicate

Each acceptance criterion must have evidence. Report executed commands and observed results, skipped or blocked checks, and any limitations. Implementation without required proof is incomplete. Compilation alone is not behavioral verification. Stop once the agreed criteria and required review are satisfied; leave local changes for the user to inspect.
