# Task workflow

## 1. Understand

Inspect status, staged and unstaged diffs, active branch, and relevant worktrees. Preserve unrelated work. Apply `codebase-design` to trace the affected entry point, owning modules, callers, and existing verification boundaries. Read the task's scoped instructions, relevant glossary entries, contracts, and ADRs. Treat source files and runtime configuration as the current implementation; report conflicts with documentation.

Done when you can name the behavior being changed, its owner, affected callers, and the checks that can detect a failure. Scale the explanation to the task.

## 2. Establish scope

For a new feature or changed product behavior, present a brief:

- Intended behavior, with a concrete example.
- Exclusions and constraints.
- Observable acceptance criteria.
- Stopping condition: the exact result after which implementation ends.
- Affected modules and interfaces.
- Verification approach and test boundaries.

Wait for explicit approval before coding. An existing detailed request still gets a concise confirmation brief. Use `grill-with-docs` when unresolved decisions require an interview. Ask only for decisions; investigate facts yourself. If answers conflict, resolve them before dependent work.

For bugs, use `diagnosing-bugs`: reproduce a real failure, reduce it, test hypotheses, repair the owner, and remove temporary instrumentation. Fix established behavior without a feature checkpoint. If the intended behavior is ambiguous, ask before choosing. Documentation and behavior-preserving refactors need a clear scope and verification plan, not an obligatory feature interview.

Done when the scope and verification boundaries are explicit and required approvals exist.

## 3. Implement

Apply `ponytail` in full mode on every implementation: understand first, reuse existing code, prefer standard libraries and native platform behavior, then write the minimum code that meets every approved criterion. Simplicity does not authorize dropping requirements, validation, security, accessibility, or resilience.

Use TDD for behavior changes through the agreed boundary. Existing boundaries are standing choices for restoring bugs; new feature boundaries are approved in the brief. Work one failing behavior and its minimal implementation at a time. Documentation, formatting, and trivial visual adjustments need appropriate verification, not obligatory new tests.

Use `architect` for substantial structural changes and `arena` when materially different designs warrant comparison. Apply relevant principle skills at their actual triggers. Add short function or key-section comments when they help a reviewer understand purpose or a decision. Avoid narrating every line.

For long tasks, use `show-me-your-work` and finish each coherent unit with a check. A long task has multiple implementation phases, a substantial migration, or work continuing while the user is away. Record agreed decisions as they settle; update `CONTEXT.md` for domain terms only. Write an ADR only for a costly-to-reverse, surprising decision with real alternatives.

Done when all approved behavior is implemented, with no unrequested expansion.

## 4. Verify and review

Follow [testing and review](testing.md). Inspect the actual result, apply `deslop`, and automatically run the normal Matt Pocock `code-review` for substantial code changes. Fix in-scope defects and repeat affected checks. Report unrelated findings without changing them. If a finding changes approved behavior or acceptance criteria, return for approval.

Done when each acceptance criterion has evidence, required reviews have no unresolved in-scope defects, and the diff contains only intended work. If required verification is blocked, report implemented versus verified and leave completion open.

## 5. Report and stop

Match the user's conversational language. Use English for repository docs, code comments, commits, and PRs. Apply `unslop`; state what changed, checks and results, and material limitations. Keep explanations short and understandable. Challenge unsupported decisions with concrete evidence and a simpler alternative where available.

Leave verified local changes for review. Stop when the agreed predicate is met. `ship` is user invoked; follow its skill only after that invocation. PRs target `dev`, are ready for review, and include a useful diagram from `show-me`. Work on feature branches for shipping; never commit to `dev` or `main`.

## Delegation and authorization

Use at most one concurrent subagent, regardless of model. Count background workers and independent review sessions in that budget. A subagent never spawns another agent or starts another independent session. Multiple perspectives may run sequentially. Delegate for bounded investigation, useful parallel work, or independent challenge. Isolate writable areas and verify returned work yourself. Use available tools and inherited models rather than inventing unsupported model names.

Within approved scope, inspect, implement, and verify autonomously. Ask before scope expansion, adding dependencies, changing public contracts or database schemas, spending money, or destructive actions. Authentication and infrastructure mutations follow [infrastructure](../../infra/AGENTS.md). Data regeneration follows [data](data.md).

Keep specs, reports, and proposed messages local. Automatic issue publication or messaging is outside the workflow. Only explicitly invoked `ship` creates PRs. Merge and deployment need separate explicit authorization. A broad instruction to continue does not erase these checkpoints.
