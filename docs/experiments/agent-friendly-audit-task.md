# Project architecture audit task

Target: the Hausy repository in this checkout, its relevant Git history, and workspace-scoped prior evidence where useful.

Produce a high-level refactor and architecture plan for team review, backed by measured observations and small executable prototypes. Use the project-local `poteto-mode`, `correct`, `architect`, `codebase-design`, `how`, and `unslop` skills. Use `recall` only for relevant context. Read their full instructions and applicable root/scoped guidance first. The operator authorized this investigation and disposable prototypes, not production refactoring.

Investigate recent commits, reverts, comments explaining workarounds, and agent instructions. Identify repeated mistake classes in code quality, performance, or bugs. A recurring class needs at least two concrete independent incidents. Do not label every fix agent-caused without evidence identifying agent provenance. Distinguish recurrence, historical repairs, current defects, and hypotheses.

For each strong class, cite exact commits and current file locations, state the ownership or data-shape issue, and prefer architectural elimination, then types, then an actionable lint, then behavioral proof, then documentation. Examine whether existing fixes already close the issue. Do not recommend reproducing obsolete fixes.

Answer factual design questions through minimal prototypes or observations. State the hypothesis, experiment, command, actual result, and limitation. Prove proposed guards can reject real historical bad behavior where practical. Benchmark only what was executed; do not describe mocked model calls as end-to-end evidence. Do not ask the user technical questions you can resolve locally. Human approval, product choices, spending, and publication remain reserved.

Compare two materially different architectural shapes under the same outcome criteria. Produce a phased plan with owner, interfaces/data shapes, dependencies, blast radius, verification, stopping conditions, and relative priority. Incorporate the property-matching prototype decisions in `docs/experiments/property-matching-prototype.md` without changing them. Broader repository improvements must be separated from the approved matching-design choices and remain proposed.

Ownership: write only `docs/experiments/agent-friendly-audit.md` and new evidence/prototype files under `docs/experiments/agent-friendly-audit/`. Keep the current application source, existing tests, main matching brief, CONTEXT.md, untracked `.claude/skills/correct/`, and unrelated changes intact. The parent integrates your report into the main brief. No commits, pushes, PRs, merges, infrastructure mutations, database truncation, new dependencies, scraping, or paid model calls. Do not delegate or start other agents.

Observable acceptance: a concise English report with evidence for each claimed recurring class, actual commands and measured results, answered technical questions, rejected alternatives, ordered high-level plan, and material unknowns. Include a compact section the parent can append directly to the matching brief. Cite actual evidence files and record any relevant memory paths/line ranges and rollout IDs used so the parent can cite them accurately.

The current user request overrides `correct` instructions to fix now or commit each class. This task ends at a reviewable plan. After user approval, the parent asks whether to execute the project-local stack or full autopilot playbook. Both remain local and preserve explicit ship/merge/deployment authorization boundaries.

Concurrency: do not spawn workers. The parent may use at most three active subagents, no more than two Claude workers and one OpenAI worker, under the operator's latest instruction.
