# Working agreement decisions

Confirmed by Nicolás on 2026-09-30 after the numbered interview. This file records rationale; operational rules live in the linked instructions.

- New features require a brief and explicit approval before implementation. Bugs restoring established behavior proceed autonomously.
- `ponytail` governs every implementation; `deslop` cleans code and `unslop` governs prose. Useful short comments are welcome.
- TDD applies to behavior changes, with practical domain/integration/component boundaries and E2E for journeys. `test-audit` also runs across the affected area after long work.
- Normal Matt Pocock review is automatic for substantial changes. The paid Claude review service is excluded.
- Delegation is limited to one concurrent subagent for every model. Workers cannot delegate. Sequential independent reviews are allowed.
- Only explicitly invoked `ship` commits, pushes, or creates PRs. Other skills produce local outputs.
- Frontend and backend guidance are separate. `frontend/DESIGN.md` is deleted; BADESIGN, current implementation, approved scope, and scoped product/accessibility rules guide UI work.
- `poteto-mode` is retained as an optional adapted playbook collection. Its local playbooks and agent wrapper follow the shared rules.
- `to-spec` is removed. `reflect` and `automate-me` retain useful local outputs without automatic publishing.
- Skill installation is pinned and reproducible. Global/shared sources remain untouched; project-local copies carry adaptations.

The writing structure follows [AI Hero's AGENTS.md guide](https://www.aihero.dev/a-complete-guide-to-agents-md) and the installed `writing-for-agents` skill: shared rules at the root, explicit conditional pointers, and checkable completion criteria.
