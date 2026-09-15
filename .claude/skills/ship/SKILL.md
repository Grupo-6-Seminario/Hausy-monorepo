---
name: ship
description: Commit the work in the current worktree, push its branch to origin, and open a pull request into dev. Use when the user runs /ship or says the task is done and ready for review.
disable-model-invocation: true
---

Ship the current worktree's work as a pull request into `dev`. Run the steps in order and
**stop and report** at the first failure — never work around a failed check.

## 1. Preconditions

```bash
git branch --show-current
gh auth status
```

- Refuse if the branch is `dev`, `main`, or empty (detached HEAD). Shipping only happens from a
  feature branch; tell the user to create one.
- Refuse if `gh` is not authenticated.

## 2. Verify

Look at `git status` and `git diff` (staged and unstaged) so you know exactly what is shipping.

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
  gh pr create --base dev --head "$(git branch --show-current)" --title "<title>" --body "<body>"
  ```

  Title: the conventional-commit summary of the whole branch. Body: what changed and why, how
  it was verified (test command and result), and anything a reviewer should look at. Follow any
  PR attribution instructions present in your context.

## 6. Report

Best effort, ignore failures (not every session runs inside Orca):

```bash
orca worktree set --worktree active --workspace-status in-review --comment "PR open: <url>" --json
```

Finish by giving the user the PR URL.
