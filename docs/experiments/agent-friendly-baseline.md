# Architecture audit baseline

Baseline revision: `7c7012072fc37476a2806b9776771a3bf58d4110`. The current branch contains local design documentation but no production-code changes from this task. These checks were executed during the architecture-plan investigation.

## Focused backend checks

Command:

```sh
go test ./internal/matching ./internal/buyer ./internal/eligibility ./internal/httpapi -count=1
```

Observed result:

```text
ok  github.com/Grupo-6-Seminario/proyecto-angus-back/internal/matching    0.638s
ok  github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer       1.284s
ok  github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility 0.942s
ok  github.com/Grupo-6-Seminario/proyecto-angus-back/internal/httpapi     4.570s
```

These package checks do not establish live model quality or disposable-database behavior.

## Current deterministic buyer benchmark

Command:

```sh
go test ./internal/buyer -run '^$' -bench '^BenchmarkTurn$' -benchmem -count=3
```

Observed result:

```text
goos: darwin
goarch: arm64
cpu: Apple M4 Pro
BenchmarkTurn-12    5636  209452 ns/op  649884 B/op  572 allocs/op
BenchmarkTurn-12    5845  206573 ns/op  649881 B/op  572 allocs/op
BenchmarkTurn-12    5709  208508 ns/op  649880 B/op  572 allocs/op
PASS
```

The median is 208508 ns/op, approximately 0.209 ms, with 649881 B/op and 572 allocations/op. There are only three samples. This is an observation, not a statistically established performance improvement.

The planner and writer are stubbed. The benchmark inventory filters neighborhoods but does not reproduce all SQL constraints. It measures deterministic buyer computation and allocations, not model latency, retrieval accuracy, or whole-system performance.

The historical metrics report in `docs/metrics/2026-10-03/REPORT.md` names an Intel Core Ultra 7 machine and a dirty revision. Its timings are not a valid direct comparator for these Apple M4 Pro runs. Any prototype comparison must execute both paths on the same machine and inputs.

## Worker dispatch

Orca launched a GPT-6.1-Sol terminal at high reasoning, but two readiness attempts failed before delivering the task. The terminal was retained as user-owned and was not closed. The audit was subsequently assigned to a native GPT-6.1-Sol high worker. The failed Orca attempts are not evidence that an audit ran.
