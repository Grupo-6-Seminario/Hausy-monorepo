---
name: show-me-your-work
description: "Keep a local evidence-backed decision trail for long, unattended, or multi-phase tasks."
---

# Decision trail

Create a local append-only TSV trail for consequential choices: timestamp, decision, reason, evidence path, verification state, and remaining uncertainty. Follow `docs/agents/workflow.md` authorization and delegation boundaries.

Update it when a decision or meaningful result occurs, not for every tool call. Use exact commands, revisions, or artifact paths where needed for reproduction. Retain rejected approaches with the evidence that rejected them. Keep logs bounded and avoid secrets or unrelated transcripts.

Before completion, inspect the trail against the actual artifact and checks. Report its path. Committing or publishing it requires explicitly invoked `ship`; unattended work does not authorize automatic review services or external side effects.
