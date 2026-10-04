# Property matching comparison

This standalone experiment compares the current buyer Agent with a proposed fit policy on fixed plans, selected committed snapshot rows and synthetic controls. It leaves application callers, database state, model-derived snapshots and the current `matching.Evaluator.Evaluate` policy unchanged.

Run from the repository root:

```sh
go test ./internal/matching ./experiments/property-matching/... -count=1
go run ./experiments/property-matching/cmd
```

The command writes `output/comparison.json`. Each case contains the actual buyer results and recorded writer packet, proposed branch rankings, retained assessments and contributions, deciding consecutive-rank reasons, exact exclusions, fallback text, word counts and literal expected-order/completeness checks. It exits unsuccessfully if an expectation fails. Source fingerprints, the actual Git revision, snapshot hashes and fixture hashes identify the run inputs.

## Ownership and comparison limits

`internal/matching.AssessFit` owns evidence assessment and weighted net preference accounting. Requirements do not contribute to the score. Preferences have weight 1, or 2 with a priority phrase present in the fixed original question. Supported and contradicted preferences contribute positively and negatively respectively. Missing evidence, conflicts, review cases and inferred hints earn zero. All requested preference weights remain in the denominator. The inherited `Criterion.Priority` field does not affect this fit policy; `Weight` and its source metadata govern preferences. Exact typed attributes use the existing deterministic classifier; prose light and quietness use the replaceable classifier interface with direct-property rubrics. This prototype supports natural_light=high and noise_level=quiet only and rejects other qualitative values explicitly. Direct typed claims outrank inferred opposites.

The experiment owns published numeric/location filtering, rental eligibility, branch grouping and final ordering. It seals the finalized record as checked JSON before producing independent projections. The single comparator supplies both the ordering and its first deciding rule. Reports, writer inputs and fallback text derive from that record.

The baseline executes `buyer.NewAgent` with a fixed intake planner, offline inventory and recording classifier/writer adapters. Its actual global URL deduplication, ten-result cap, quality-before-eligibility order and deterministic writer fallback remain observable. The proposed path preserves every branch membership and ranks each branch with its own rubric and denominator. A shared URL can appear in several branches. The overview counts distinct shown properties. Different branch rubrics do not establish a direct pooled ranking improvement.

Both paths use frozen assessment answers for common light questions. The current buyer assesses natural light semantically and handles noise preferences through parsed attributes. The proposed noise rubric adds recorded support and contradiction assessment. Noise-related changes combine this capability difference with ranking policy changes. They must not be presented as isolated scoring benefits. The command does not measure classifier accuracy.

The in-memory inventory applies the fixture's published gates without query caps. Real rows must have a passed review whose `quality.Fingerprint` matches the normalized listing. Eligibility rows and published pets rules are reused. Retrieval is complete only within the selected fixture population. This does not exercise PostgreSQL, archive state, paging, planner inference or live CABA inventory. Numeric currency boundaries remain explicit. Proposed price sorting requires a declared currency; baseline behavior remains intact.

Frozen cases and expectations live in `fixtures/cases.json`. `fixtures/freeze.json` identifies the development and held-out split and records input corrections discovered before valid execution. Preflight rejects missing answers, bad evidence references and undeclared simulated failures. The only expected incomplete ranking is the explicit provider-failure case. Provider failure, missing evidence, conflict and review remain different assessment states.

## Explanation checks

The deterministic fallback and optional writer receive the same compact Spanish conclusions derived from the finalized record. Each includes at most three properties, exact branch counts, relevant eligibility conditions and exclusion/relaxation facts. Comparisons are made only when the peer appears in the explanation packet. A lone or last explained property gets an individual fit explanation. Targeted listing questions have a separate topic and answer directly, preserving published decimal values.

Two free rewriting attempts changed the facts and failed live review. Their raw outputs remain in `output/live-comparison.json` and `output/live-comparison-v2.json`. The closed writer now supplies complete backend sentences as ordered blocks. The local model may return only block IDs. Exact coverage and order are validated before rendering the authoritative sentences. Foreign, dropped, duplicate, reordered or malformed selections use deterministic fallback. Raw selections and validation failures are retained. This prevents new model claims and shows no added value from the model. The selected facts and backend wording still need independent review.

Paired local generation uses the existing buyer writer on its actual packet and the prototype writer through the same local provider:

```sh
go run ./experiments/property-matching/cmd -live \
  -cases weighted-net,mandatory-alternatives,lone-result \
  -output experiments/property-matching/output/live-comparison.json
```

Only loopback endpoints without URL userinfo are allowed. `-local-url` and `-model` select an already-running local provider and installed model. No inference runs by default. Selection status, raw model response, selection failure, rendered text, word counts and actual elapsed latency are recorded. Accepted selections have validated_blocks status; they do not represent model-authored prose. Tokens and cost remain null because the existing provider-neutral client exposes neither. Empty, failed or invalid proposed selections use the same deterministic fallback. A failed baseline generation is labeled unavailable.

Keep failed live outputs as evidence. Do not tune held-out cases. Run all fixed cases for the final comparison after the development path and independent review are stable.

## Benchmarks

Stop local inference before timing. Run repeated paired workloads on the same machine:

```sh
go test ./experiments/property-matching -run '^$' \
  -bench BenchmarkComparison -benchmem -benchtime=300ms -count=5 \
  > experiments/property-matching/output/benchmarks.txt
```

Each workload uses the same case, plan, candidates, qualification and frozen provider answers. Baseline timing includes actual buyer orchestration and deterministic fallback. Proposed timing includes fit assessment, branch finalization, sealing, report projection and fallback construction. Loading snapshots occurs outside the timed loop. Planner/provider inference, SQL and generated writer calls are excluded. Retained evidence and projection overhead are charged to the proposed path. Different retained data and branch behavior mean these are operation costs, not proof of an equal-work speed improvement.

Focused tests cover weighted accounting, direct exact attributes, inferred hints, required contradictions, unavailable/review/conflicting assessments, source priorities, literal development orders, targeted questions, retained-record mutation isolation, compact writer facts, publication-backed pets eligibility, fixture preflight, false relaxation promises, closed writer selections and loopback URL validation. The parent completed the full-package checks, independent review, held-out run and [results report](../../docs/experiments/property-matching-results.md). Final artifacts live under `output/`; failed trials are preserved separately.

Replay the observed free-writer failure census with:

```sh
python3 experiments/property-matching/scripts/audit_free_writer.py
```

This counts known observed failure phrases by actor. It is a narrow evidence replay, not a semantic judge or an accuracy metric.
