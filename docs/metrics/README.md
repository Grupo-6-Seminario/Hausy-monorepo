# Metrics

Dated snapshots of the backend's package design, code complexity and performance, so the effect of
a feature or an integration can be measured against what came before it.

| Snapshot | Revision | Taken for |
| --- | --- | --- |
| [2026-09-28](./2026-09-28/REPORT.md) | `3559c82` | Baseline before the December MVP work |

## Taking a snapshot

Commit first: the report records `git describe --dirty`, and a dirty revision cannot be rebuilt.

```bash
docker compose up -d
docker exec hausy-db createdb -U hausy hausy_test   # once; the Postgres benchmark empties it
HAUSY_TEST_DATABASE_URI=postgresql://hausy:hausy@localhost:5432/hausy_test \
  go run ./cmd/metrics -bench -out docs/metrics/$(date +%F)
```

Without `-bench` it takes seconds and measures only design and complexity. Without
`HAUSY_TEST_DATABASE_URI` the Postgres benchmark skips. `-count` sets the runs per benchmark (10).
Add a row to the table above.

| File | Contents |
| --- | --- |
| `REPORT.md` | Summary, the three charts, and their tables |
| `packages.csv` | A, I, D, Ca, Ce, types, interfaces, third-party imports, complexity per package |
| `functions.csv` | Cyclomatic complexity of every function, most complex first |
| `main-sequence.svg` · `distance.svg` · `complexity.svg` | Charts; light and dark follow the viewer |
| `bench.txt` | Raw `go test -bench` output, in the format benchstat reads |

## Comparing snapshots

Design and complexity do not depend on the machine. Diff the CSVs:

```bash
git diff --no-index --word-diff docs/metrics/2026-09-28/packages.csv docs/metrics/<new>/packages.csv
```

Benchmarks do. Compare only runs from the same machine (the `cpu:` line in `bench.txt`):

```bash
go run golang.org/x/perf/cmd/benchstat@latest docs/metrics/2026-09-28/bench.txt docs/metrics/<new>/bench.txt
```

If the baseline was taken on another machine, re-run it on yours from the baseline's revision:

```bash
git worktree add ../baseline 3559c82
(cd ../baseline && go run ./cmd/metrics -bench -out /tmp/baseline)
```

benchstat marks a change as significant only when the runs support it; `~` means no measurable
difference.

## What the numbers mean

### Abstractness, instability, distance

Robert C. Martin's package metrics, adapted to Go:

- **Ca** (afferent) counts the module's packages that import this one; **Ce** (efferent), the
  module's packages this one imports. Only non-test files count, and `experiments/` and
  `cmd/metrics` are left out. The standard library is ignored. Third-party imports are reported
  apart, since they are coupling this ratio does not see.
- **Instability** `I = Ce / (Ca + Ce)`. 0: everything depends on it and it depends on nothing.
  1: the reverse.
- **Abstractness** `A = interfaces / named types` declared in the package. Go has no abstract
  classes, so an interface is the only abstract type. Aliases are not types; function types count
  as concrete.
- **Distance from the main sequence** `D = |A + I − 1|`. A stable package should be abstract and an
  unstable one concrete; D measures how far a package is from that balance. Near (0, 0) is the
  **zone of pain**: concrete, and hard to change because many depend on it. Near (1, 1) is the
  **zone of uselessness**: abstractions nobody uses.

Go shifts the reading. Interfaces live where they are consumed (`buyer.Inventory`,
`buyer.Planner`, `pipeline.Sink`), not where they are implemented, so a stable package of data
types and pure functions scores A = 0 and lands in the zone of pain by construction. Martin's own
qualification applies: the zone of pain hurts only for packages that change. Read D together with
how often the package changes.

### Cyclomatic complexity

McCabe's count of independent paths: one, plus one per `if`, `for`, `range`, non-default `case`,
non-default `select` case, `&&` and `||`. It is counted exactly as gocyclo v0.6.0 counts it; the
baseline matches gocyclo on all 285 functions. Bands: 1–10 simple, 11–20 moderate, 21–50 complex,
over 50 untestable. McCabe's recommended limit is 10.

### Benchmarks

All run over the committed inventory in `data/` (300 listings), so every snapshot measures the same
input. None call a model.

| Benchmark | Measures |
| --- | --- |
| `pipeline.Parse` | Normalizing the 300 scraped listings, model stubbed: the deterministic half of `listings parse`, where a new portal adds its parser |
| `pipeline.Load` | Decoding the parsed listings as `listings load` does, without the database |
| `eligibility.Assess` | One searcher against the rules of all 300 listings |
| `eligibility.Relaxations` | The zero-results engine over all 300 listings |
| `buyer.Turn` | One turn of a new conversation between its two model calls: eligibility, relaxations and ordering over all 200 listings of Palermo and Congreso, served from memory |
| `store/postgres.Candidates` | The hard-filter query for Palermo and Congreso up to ARS 900,000 over the loaded inventory (92 candidates) |

Not measured yet: model latency (planner, writer, clarification, matching), the HTTP layer and the
frontend.

## Baseline, 2026-09-28

21 packages, mean D 0.51, nine packages with D > 0.7. 285 functions, mean complexity 6.09, median
4; 35 functions (12.3%) are over 10 and none over 50. Benchmark runs vary by 4% or less (benchstat).

![Abstractness vs instability](./2026-09-28/main-sequence.svg)

**The volatile packages sit near the main sequence.** Commits touching non-test code since the
first commit (2026-09-01), next to D:

| Package | D | Commits |
| --- | ---: | ---: |
| `internal/buyer` | 0.06 | 14 |
| `cmd/hausy` | 0.00 | 11 |
| `internal/store/postgres` | 0.25 | 9 |
| `internal/intake` | 0.21 | 7 |
| `internal/httpapi` | 0.02 | 7 |
| The nine packages with D > 0.7 | 0.75–1.00 | 1–4 each |

The packages in the zone of pain (`listing`, `eligibility`, `search`, `jev`, `logging`, `auth`,
`agency`, `matching`, `llm`) are the stable, concrete ones Go puts there, and they change rarely.
Two are worth watching: `listing` has nine dependents and `eligibility` seven, so a change to their
types spreads that far. The MVP names qualification as the only swappable part, yet every caller
of `eligibility` depends on its concrete functions (A = 0). If that swap is built, an interface at
the consumer is the move, and D will show it.

`pipeline` sits above the line (A 0.67, I 0.75): four interfaces and one dependent,
`cmd/listings`. Each interface has one production adapter (`listing.Parser` for the parser,
`postgres.Store` for the three sinks) plus test fakes.

![The 15 most complex functions](./2026-09-28/complexity.svg)

**Complexity concentrates in the conversation layer.** Four of the seven most complex functions are
in `internal/buyer`: `HandleClarification` 49, `handlePipeline` 35, `questionImpact` 28,
`prepareClarification` 27. `HandleClarification` is one path short of the untestable band.
`matching.(*Evaluator).Evaluate` (44) and `intake.(Jev).Plan` (42) follow. These are the
refactoring candidates, and the ones to watch as clarification features land.

**The candidates query is about 99% of a turn's deterministic time.**

| Benchmark | Time/op | Allocs/op |
| --- | ---: | ---: |
| `store/postgres.Candidates` | 24.97 ms | 13,016 |
| `pipeline.Load` | 5.27 ms | 27,489 |
| `pipeline.Parse` | 4.84 ms | 45,302 |
| `buyer.Turn` | 203 µs | 572 |
| `eligibility.Relaxations` | 15.1 µs | 175 |
| `eligibility.Assess` | 9.7 µs | 88 |

`Store.candidates` selects the matching IDs and loads their rules, then calls `ByURL` once per
candidate, and `ByURL` makes two queries (the row, then its attributes): 92 candidates cost 186
round trips. Ranking, eligibility and relaxations over 200
listings in memory take 0.2 ms. The query's cost grows with every match, and each round trip costs
more against a remote database than against local Docker. Loading the listings in one query
(`WHERE id = ANY($1)`) is the obvious fix, and this benchmark is its baseline. Eligibility is not a
bottleneck: 32 ns per listing.

Model calls are outside all of this, and they dominate: the Jev planner alone may take up to 6 s
before its fallback. A turn costs its model calls plus about 25 ms of deterministic work.
