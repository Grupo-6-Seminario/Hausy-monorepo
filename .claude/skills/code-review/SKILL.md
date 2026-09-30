---
name: code-review
description: "Review substantial code changes automatically, or a user-requested branch, PR, or working diff, along separate Standards and Spec axes."
---

# Code review

Use the normal local reviewer. Read `docs/agents/testing.md` and the affected scoped instructions. Pin a user-specified base when supplied; otherwise inspect the merge-base with `dev`. Include staged and unstaged task changes for working-tree review. Record the exact base and changed files; an empty diff is a reportable result.

Find the approved brief or matching local spec and applicable standards. If the spec is missing, report that limitation rather than inventing requirements or demanding an issue tracker. Fetch external issue content only when the user supplies it as a source; keep all findings local.

Run two independent axes within the delegation policy in `docs/agents/workflow.md`: the parent handles Standards and one subagent handles Spec, or vice versa. If delegation is unavailable, review both sequentially and disclose the lack of independence. Workers do not delegate.

Standards checks documented conventions, meaningful complexity, duplication, naming, boundary placement, and speculative abstractions. Treat code smells as heuristics; the repo's actual standards prevail. Skip findings already enforced by tooling unless the check is failing.

Spec checks missing or incorrect behavior, unrequested additions, acceptance evidence, and failure paths against the approved brief. Cite the relevant requirement and affected code for each actionable finding.

Report Standards and Spec separately, with evidence and confidence. Do not invoke paid review services, publish comments, change issue status, or create a PR. The implementing parent fixes in-scope defects and reruns affected checks under the completion workflow.
