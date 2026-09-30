---
name: principle-build-the-lever
description: "Use a rerunnable tool when repeated manual work or verification would be less reliable."
---

# Build a useful tool

Reuse an existing command, script, or runner first. Build the smallest rerunnable tool when repeated manual edits or checks create a real reliability cost. A one-off straightforward edit does not require a framework, codemod, or generator.

Verify the tool against actual inputs and failure cases, and retain it when it helps reproduce the approved result. Respect `ponytail` and the scope in `docs/agents/workflow.md`.
