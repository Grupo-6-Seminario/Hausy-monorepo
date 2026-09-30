---
name: ship
description: Commit the work in the current worktree, push its branch to origin, and open a pull request into dev. Use only when the user explicitly invokes ship or /ship.
disable-model-invocation: true
---

Ship the current worktree's work as a pull request into `dev`. Run the steps in order and
**stop and report** at the first failure — never work around a failed check.

## 1. Preconditions

Confirm the user explicitly invoked `ship`. Task completion, ready-for-review wording, or another skill's final step is not invocation. Read `docs/agents/testing.md` and run applicable domain checks; skipped required proof blocks shipping.

```bash
git branch --show-current
gh auth status
```

- Refuse if the branch is `dev`, `main`, or empty (detached HEAD). Shipping only happens from a
  feature branch; tell the user to create one.
- Refuse if `gh` is not authenticated.

## 2. Verify

Look at `git status` and `git diff` (staged and unstaged) so you know exactly what is shipping.

Run all applicable completion checks from `docs/agents/testing.md`, including frontend checks when frontend behavior changed.

If any Go code changed on this branch (`git diff --name-only origin/dev...HEAD` plus uncommitted
files), run `go test ./...`. If tests fail, stop — do not commit or push red code.

## 3. Commit

If there are uncommitted changes:

- Stage only files that belong to this task. Never stage `.env*` (other than `.env.example`),
  credentials, keys, or build artifacts; if one shows up, stop and ask.
- Write a conventional-commit message matching the repo history (`feat(scope): ...`,
  `fix(scope): ...`) that says what changed and why.
- Follow any commit attribution instructions present in your context.

If the tree is clean and `git log origin/dev..HEAD` is empty, there is nothing to ship — say so
and stop.

## 4. Push

```bash
git push -u origin HEAD
```

Never force-push. If the push is rejected, stop and report why. Confirm the remote has the
commit: `git status -sb` must show no `ahead` count.

## 5. Pull request

```bash
gh pr view --json url,state,baseRefName
```

- **An open PR already exists for this branch:** the push updated it. Report its URL. If its
  base is not `dev`, say so rather than changing it.
- **No open PR:** create one — ready for review, not a draft:

  ```bash
  gh pr create --base dev --head "$(git branch --show-current)" --title "<title>" --body-file <prepared-body-file>
  ```

  Title: the conventional-commit summary of the whole branch. Body: what changed and why, a
  diagram, how it was verified (test command and result), and anything a reviewer should look
  at. Follow any PR attribution instructions present in your context.

  **Diagram (required):** from the branch diff (`git diff origin/dev...HEAD`), draw the smallest
  useful view (use the `show-me` skill if installed) and paste it into the body under a
  `## Diagram` heading — a Mermaid block, call tree, file tree, or `diff` sketch. Only use
  formats that render in GitHub markdown; never an HTML file.

  If the PR already exists and has no `## Diagram` section, add one with
  `gh pr edit --body-file`.

Only create or update the PR; merging and deployment require separate explicit authorization.

## 6. Report

Best effort, ignore failures (not every session runs inside Orca):

```bash
orca worktree set --worktree active --workspace-status in-review --comment "PR open: <url>" --json
```

Finish by giving the user the PR URL.
