# Intake spike: Jev vs the local buyer agent

Question: can Jev turn a searcher's messages into a search plan more reliably
and faster than the local model's tool loop?

```bash
go run ./experiments/intake -system=jev    # internal/intake Jev planner; needs AI_GATEWAY_API_KEY
go run ./experiments/intake -system=qwen   # internal/intake local-model planner (one call)
```

Since specs/002 the runner drives the production planners through `intake.Planner`, so turn rules
(frozen plan, default rent) apply exactly as in the app. The first results below came from the
earlier spike code (Jev v1/v2, Qwen tool loop) and are kept as history.

`cases.json`: 20 conversations (6 multi-turn), labeled before either system ran.
Scoring compares the final plan: expected slots (with accepted alternatives),
optional items, and extras.

- **Qwen:** the production `buyer.DefaultAgent` + Postgres. The plan is the
  query that actually ran, read from `TurnResponse.Requirements`.
- **Jev:** barrio names and numbers are extracted deterministically. Jev decides
  their roles, the operation, the turn intent, and each attribute. v2 sends one
  request per turn.

## Results — 2026-09-23, one run each

| | Qwen tool loop | Jev v2 intake |
| --- | --- | --- |
| Exact plans | 13/20 | 18/20 |
| Expected slots found | 49/74 | 74/74 |
| Wrong extra items | 1 | 2 |
| Failed turns | 5 (hit the 4-step tool limit) + 1 without a search | 0 |
| Turn intent | n/a | 20/20 |
| Latency per turn | p50 23 s, p90 30 s (the full turn, including the reply) | p50 5.5 s, p90 13.5 s (intake only) |

Jev turns with no retry took 0.5–4.1 s. Retries: 26 over 26 turns (429s and
503s from Gateway), with up to 45 s of backoff. Gateway reported cost 0.

Jev errors: "¿el primero tiene balcón?" added
`pref:outdoor_space=balcon`; "hasta 900 mil" with no operation became `venta`.

Relabel: `drop_parking` originally expected the cochera requirement to be
dropped; after the run the product owner confirmed that rioplatense "olvidate de
la cochera" means "no cochera", so `excl:amenity=cochera` is accepted (Jev went
from 17/20 to 18/20; Qwen unchanged).

Caveats: n=20, one run each, labels by one person. v1 (three requests per turn
with per-pair questions) failed on provider errors; v2 was designed after seeing
v1 on the same cases. Qwen latency includes writing the reply; Jev latency does
not. A Jev pipeline still needs one model call to explain its results.

## Production planners (`internal/intake`) — 2026-09-23

There are 23 cases (3 added: scoped branches, sort, qualification volunteered in chat). Each runs
through `intake.Planner`. Jev's client used 6 retries starting at 1 s, to separate accuracy from
Gateway errors.

| | Jev | Qwen, single call |
| --- | --- | --- |
| Exact plans | 15/23 | 17/23 |
| Exact among answered | 15/16 | 17/23 |
| Gateway errors after retries | 7 (503 / timeout) | 0 |
| Turn latency | p50 4.7 s, p90 68 s | p50 5.3 s, p90 32 s |

This run found three bugs, now fixed with regression tests in `internal/intake`:
- The Qwen planner copied the prompt's `0` fields into real bounds.
- The Qwen planner invented `income_band` for users who never mentioned income.
- The Jev planner produced a search branch for a greeting.

Still open: Qwen confuses dormitorios with ambientes once and "que no sea interno" once. The
numbers above predate the income fix.

2026-09-24: Jev read an unstated operation with a peso price ("…hasta 900 mil", with or without
"tengo garantía propietaria") as `venta`, top choice at 0.38–0.58, in 7 of 8 live runs. The plan now
keeps a sale only when a user message says so (`saleCue` in `internal/intake/planner.go`); the
same three cases plus a `comprar` control went 12/12 afterwards. `op=alquiler` is now expected on
implied rentals, and `implied_rent_guarantee` is the reported conversation. Full Jev run after the
fix, 24 cases, one run: 24/24 exact.
