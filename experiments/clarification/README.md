# Clarification proposal evaluation

Run `go run ./experiments/clarification` with the same `LOCAL_LLM_URL`,
`LOCAL_LLM_MODEL`, and `LOCAL_LLM_TOKEN` as the app. Use `-only=<case-id>` to
inspect one label. The runner makes one planner call and one proposal call per
case, reads no database, and prints pass counts and latency without user text.

`cases.json` labels whether the proposer should ask a search question, ask an
optional qualification question, explain an unsupported condition, or proceed.
This evaluates the proposal seam. Buyer integration tests cover capability,
result-impact checks, answer application, and stale question IDs. Compare the
runner's false positives, missed material ambiguities, and wait time with
real search-turn logs before rollout; set numerical release thresholds from a
baseline, not from these eight fixtures alone.
