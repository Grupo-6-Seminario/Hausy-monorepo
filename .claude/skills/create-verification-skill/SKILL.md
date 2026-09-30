---
name: create-verification-skill
description: "Create a project-local live verification skill when explicitly requested."
disable-model-invocation: true
---

# Create verification skill

Discover how to launch, isolate data, drive the actual interface, collect evidence, and clean up only processes/resources you created. Define an affected feature map and observable completion criteria.

Write a project-local skill with `writing-for-agents`. Use the agent-neutral `.agents/skills/` installation convention and tracked project overrides rather than inventing a Cursor-only dependency. Reuse existing runners and require approval for new dependencies or external mutations.

Execute one representative journey end to end with designated disposable data. Record launch commands, evidence, limitations, and cleanup. Done when another session can reproduce that proof using the skill. Follow `docs/agents/workflow.md` and leave changes local.
