# Local queue execution

Read `docs/agents/workflow.md` and the task's scoped instructions first.

This repo does not use automatic PR or merge queues. Translate the request into sequential local units with approved scope and evidence. Maintain one concurrent subagent with no nested delegation. Stop at verified local output; invoke `ship` only on explicit user instruction and request separate merge authorization.

Complete when the requested result has direct evidence and unresolved limits are reported. Keep outputs local under the shared authorization rules.
