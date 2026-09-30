---
name: poteto-mode
description: "Use the adapted Poteto playbook collection when explicitly requested."
disable-model-invocation: true
---

# Poteto mode for Hausy

This optional collection adds task-specific investigation and execution playbooks to the shared workflow. Read `docs/agents/workflow.md` and the scoped instructions first. Choose the matching playbook below and use its completion criterion. Ordinary tasks follow the shared workflow without invoking this mode.

Use `ponytail` for implementation, `unslop` for prose, and the relevant principle leaf when its actual trigger applies. Inspect the affected flow before choosing the shape. Useful short comments are welcome. Apply normal review and artifact verification as defined in `docs/agents/testing.md`.

Follow shared feature approval, scope, delegation, and publication rules. These playbooks do not supply authority to commit, publish, merge, authenticate, spend, deploy, or delete data. Use available tools and inherited models. A worker cannot delegate.

## Choose a playbook

| Task | Playbook |
| --- | --- |
| Read-only investigation | [Open](playbooks/investigation.md) |
| Bug fix | [Open](playbooks/bug-fix.md) |
| Performance issue | [Open](playbooks/perf-issue.md) |
| Measured improvement | [Open](playbooks/hillclimb.md) |
| Runtime diagnosis | [Open](playbooks/runtime-forensics.md) |
| Feature | [Open](playbooks/feature.md) |
| Refactoring | [Open](playbooks/refactoring.md) |
| Prototype | [Open](playbooks/prototype.md) |
| Visual parity | [Open](playbooks/visual-parity.md) |
| Skill authoring | [Open](playbooks/authoring-a-skill.md) |
| Agent behavior evaluation | [Open](playbooks/eval.md) |
| PR inspection | [Open](playbooks/babysit.md) |
| Shipping | [Open](playbooks/shipping.md) |
| Autonomous local run | [Open](playbooks/autonomous-run.md) |
| Bounded multi-phase coordination | [Open](playbooks/orchestrate.md) |
| Local queue execution | [Open](playbooks/autopilot-full.md) |
| Local dependent sequence | [Open](playbooks/autopilot-stack.md) |
| Resume work | [Open](playbooks/session-pickup.md) |
| Pause safely | [Open](playbooks/pause-safely.md) |
| Multi-phase plan | [Open](playbooks/multi-phase-plan.md) |
| Worktree cleanup | [Open](playbooks/worktree-cleanup.md) |
| PR delivery | [Open](playbooks/opening-a-pr.md) |
| Captured trace diagnosis | [Open](playbooks/trace-forensics.md) |
