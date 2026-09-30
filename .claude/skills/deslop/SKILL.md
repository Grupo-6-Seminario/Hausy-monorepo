---
name: deslop
description: Remove AI-generated code slop and clean up code style
---

# Remove AI code slop

Check the task diff against its actual base, normally the merge-base with dev, including staged and unstaged changes and remove AI-generated slop introduced in the branch.

## Focus Areas

- Redundant or inaccurate comments; preserve useful short function and key-decision explanations
- Defensive checks or try/catch blocks that are abnormal for trusted code paths
- Casts to `any` used only to bypass type issues
- Deeply nested code that should be simplified with early returns
- Other patterns inconsistent with the file and surrounding codebase

## Guardrails

- Keep behavior unchanged unless fixing a clear bug.
- Prefer minimal, focused edits over broad rewrites.
- Keep the final summary concise (1-3 sentences).
