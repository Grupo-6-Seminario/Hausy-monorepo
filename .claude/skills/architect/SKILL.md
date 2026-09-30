---
name: architect
description: "Design substantial structural changes or unclear module and interface ownership before implementation."
---

# Architecture

Read the affected flow using `how` and `codebase-design`. Name the data shape, public interfaces, ownership, and invariants before code. Respect the feature approval checkpoint in `docs/agents/workflow.md`.

For a substantial structural choice with meaningful alternatives, use `arena` to compare materially different shapes. For a bounded change with an established pattern, reuse the pattern and state why another design exercise adds no value. Sketch types, signatures, call flow, and verification boundaries. Keep provider and transport seams deep and the interface small.

Stay available through implementation to check whether the actual shape fits the approved intent. Replace a demonstrably wrong design at its owner rather than adding symptom patches. Done when the chosen shape satisfies the approved behavior and its verification criteria. Keep outputs local.
