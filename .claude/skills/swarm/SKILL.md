---
name: swarm
description: "Coordinate partitioned work or independent scrutiny when the user explicitly requests swarm or parallel agent work."
disable-model-invocation: true
---

# Bounded swarm

Partition the task into explicit scopes and done predicates. Follow `docs/agents/workflow.md`: at most one concurrent subagent, no delegation by workers, parent-owned integration. Use sequential partitions when more coverage is needed. Isolate writable outputs before concurrent work.

For a race, agree the stopping predicate before running. For a comparison, pin inputs, revisions, commands, and measurement conditions. Inspect each output and rerun central claims against the actual artifact. Consolidate pass, issues, and blocked work without treating a worker's report as proof. Keep all outputs local.
