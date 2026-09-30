# Project skill setup

The shared baseline is pinned in [skills.lock.json](skills.lock.json). Project adaptations live in [skill-overrides](skill-overrides/). Optional personal skills stay outside this baseline. Review changes to the baseline as changes to future agent behavior.

## Restore a fresh checkout

Run `python3 scripts/setup-agent-skills.py` from the repo root when the user authorizes setup. Python 3.11+ and Git are prerequisites. If `python3` is unavailable, use the cross-platform `uv run --python 3.11 --no-project scripts/setup-agent-skills.py` after installing uv; the same prefix runs the checker and verification script. It downloads exact source revisions, copies the selected skills into the ignored `.agents/skills/` directory, overlays Hausy adaptations, and supplies Claude-facing local aliases. Existing tracked Claude skills remain canonical where configured. Windows installations copy instead of requiring symlinks. Verification normalizes text line endings. Setup adds newly generated aliases to the checkout's Git exclude file, preserving the visibility of pre-existing personal files.

Run `python3 scripts/check-agent-instructions.py --installed` afterward. To exercise fresh installation, repeatability, missing outputs, personal-skill preservation, Windows line endings, and drift rejection in a disposable checkout, run `python3 scripts/verify-agent-setup.py`. The setup command fails rather than silently replacing an existing different skill; use `--refresh` only when explicitly updating the project's baseline installation. It preserves unrelated skills and global installations. Installing a skill is not authorization to execute its external side effects.

## Missing or incompatible skills

Locate project-local skill files first in `.agents/skills/<name>/SKILL.md`, then `.claude/skills/<name>/SKILL.md`. Read the selected skill in full. If a required skill is missing, report it, follow the workflow documented here where possible, and ask before installation. A missing tool or skill must not lead to invented commands, models, or skipped acceptance criteria.

Translate tool calls to the available agent platform. Enforce one concurrent subagent and no nested delegation even when an upstream reference suggests fan-out. Use inherited model settings. Project workflows govern adapted skill behavior; global copies do not override project rules.

Use [skill routing](skills.md) for activation. Read only selected skills, not the entire inventory each task. `writing-for-agents` governs instruction writing. Optional `poteto-mode` uses the local adapted playbooks, while ordinary work follows [workflow](workflow.md).

## Maintenance

Change a pin or override deliberately, rerun setup in a disposable checkout, verify routing/link coverage, and exercise affected workflow branches in a fresh agent session. Keep findings local. Only user-invoked `ship` publishes a PR.

Matt Pocock's issue-tracker setup wizard is excluded here: publishing/triage flows are not part of this repo workflow. Scope briefs and specs remain local unless the user gives a separate explicit publication instruction outside these skills.
