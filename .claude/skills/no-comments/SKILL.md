---
name: no-comments
description: "Review comments for usefulness when explicitly requested, preserving concise function and key-decision explanations."
disable-model-invocation: true
---

# Comment review

Read the comments and the code they explain. Keep useful short comments above functions and important logic, especially purpose, surprising constraints, or tradeoffs. Remove inaccurate, redundant, or line-by-line narration only within the requested scope.

Where a comment claims an invariant, inspect whether code actually enforces it. Prefer a structural enforcement only when it improves the scoped implementation. Report uncertain claims instead of deleting them or changing unrelated behavior. Use one independent comment reviewer if useful under `docs/agents/workflow.md`; that reviewer does not delegate.

Verify behavior-preserving edits, apply `deslop`, and leave local changes for review.
