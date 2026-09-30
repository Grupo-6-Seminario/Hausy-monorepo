# Shipping

Read `docs/agents/workflow.md` and the task's scoped instructions first.

Shipping is performed only by explicitly invoked `ship`. Follow its preconditions, verification, selective staging, push, and ready PR into `dev`. This playbook does not commit, create PRs, merge, or deploy independently. Return to local completion if `ship` was not invoked.

Complete when the requested result has direct evidence and unresolved limits are reported. Keep outputs local under the shared authorization rules.
