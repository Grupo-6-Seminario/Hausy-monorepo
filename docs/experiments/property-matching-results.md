# Property matching comparison results

Status: standalone prototype complete and independently reviewed. The application path remains unchanged.

Baseline revision: `7c7012072fc37476a2806b9776771a3bf58d4110`. Branch: `refactor/property-matching-explanations`. Scope and stopping criteria are in the [approved brief](property-matching-prototype.md).

## Execution

The user selected stack on 2026-10-04. GPT-6.1-Sol at high reasoning owns the fit operation and standalone comparison implementation. Claude Opus 5.5 at high effort owns an independently worked oracle and Spec review. The parent owns Standards review, integration and final verification. Workers do not delegate or publish changes.

The native OpenAI worker is `/root/matching_stack_implementation`. Orca run `run_2b27d5e8a6ff` started Claude task `task_597c8ff743dc`, dispatch `ctx_1dad8a1787d0`. Its launch receipt reports requested and effective `claude-opus-5-5`, effort `high`, with an observed turn start. Neither worker owns the other worker's writable files.

Claude initially reported an expired login and did not run verification. The user restored sign-in. The same worker then began reading the approved brief and existing baseline code. Launch configuration alone was not treated as proof that verification ran.

The local endpoint check `GET http://127.0.0.1:8000/v1/models` initially returned connection refused. The parent found the installed `omlx` executable and cached `Qwen3.5-9B-4bit` weights, then started an isolated loopback server with a temporary base directory, one concurrent request, the safe memory guard and cache disabled. Hugging Face and Transformers offline modes prevented model downloads. Model discovery now returns `Qwen3.5-9B-4bit`. The completed trials and final closed-writer run are reported below. The parent stopped this temporary server after inference and before benchmarking; cached weights and user services were preserved.

## Dependent units

1. Retain evidence assessments and contributions in a separate matching fit operation. Preserve the current evaluator and app behavior.
2. Finalize grouping, explicit sort and deciding reasons once. Compare with the real current buyer on identical frozen inputs.
3. Produce compact Spanish explanations and deterministic failure fallback from the retained record.
4. Execute the independent cases, repeated comparable benchmarks, full Go checks and independent review. Record implemented, measured and unverified behavior separately.

Broader SQL test migration and application adoption are outside this implementation. The user has since authorized a local commit for this branch. Pushes, PRs, merges and deployment retain their separate authorization boundaries.

## Initial live pass

The first paired local Qwen run generated six responses through the existing local adapter. The baseline used the actual `buyer.LocalWriter`; the proposed writer used its own prompt and retained record projection. Outputs are preserved in [live-initial.json](../../experiments/property-matching/output/live-initial.json).

| Development case | Baseline words | Proposed words | Baseline latency ms | Proposed latency ms |
| --- | ---: | ---: | ---: | ---: |
| weighted-net | 122 | 105 | 13043.309 | 10257.491 |
| mandatory-alternatives | 145 | 86 | 8355.797 | 8203.233 |
| lone-result | 106 | 109 | 5566.439 | 5187.075 |

This pass failed explanation fidelity. In `mandatory-alternatives`, the proposed text wrongly described noise as an advantage for a searcher who preferred silence. The weighted case used internal points terminology and omitted the explicitly prioritized light difference between the second and third properties. The lone result used internal classification language. Shorter output therefore did not meet acceptance.

A second development run removed score records and full listing descriptions from the writer input. Independent held-out checks remained reserved during both repairs. Token use and cost are unknown because the reused provider-neutral adapter does not expose that telemetry. The timing samples are observations, not a general latency improvement claim.

## Second live pass and writer decision

[The second live run](../../experiments/property-matching/output/live-comparison-v2.json) used only Spanish facts and deciding reasons prepared by the backend. It still failed fidelity.

| Development case | Baseline words | Proposed words | Proposed problem |
| --- | ---: | ---: | --- |
| weighted-net | 122 | 91 | Changed a listing claim into guaranteed silence and omitted the explicit priority behind the second-versus-third comparison. |
| mandatory-alternatives | 145 | 118 | Wrote "El aviso contradice silencio, así que no es ruidoso", reversing the source meaning. |
| lone-result | 106 | 50 | Added internal ranking-completeness commentary. |

Both attempts assumed that free paraphrasing would preserve the backend's meaning. The failures were in generated prose. The retained assessments and deterministic comparison reasons did not contain the reversed noise claim. Removing internal data reduced length but did not prevent rewriting from changing a fact.

The final prototype therefore confines the model to references to backend-approved text blocks. The backend validates their coverage and order, then renders the exact text. A missing, repeated, foreign or reordered reference uses the deterministic fallback. This tests the LLM boundary while keeping published claims and ranking explanations under backend control. It does not establish useful value from retaining the model call. If these fixed blocks satisfy the product, removing that call is the simpler adoption option.

## Final comparison

The [final command output](../../experiments/property-matching/output/comparison.json) passes all 18 literal order and completeness expectations, including three cases held out until code freeze. The independent Claude oracle evaluated 35 controls, including its own three held-out cases and real Palermo slices. The [review](property-matching-review.md) records corrected oracle inputs separately from implementation repairs. Portable controls are preserved under [verification](property-matching-verification/run.py). The parent reran them successfully on the final code. One targeted-question check needed a disclosed lexical assertion correction; the original harness and failed replay remain preserved.

The reusable `AssessFit` operation retains assessments, provenance, contributions and weighted preference accounting. The experiment gates published constraints, assesses rental eligibility and creates branch-specific order and comparison reasons with one comparator. Immutable projections prevent a consumer from changing the authoritative evidence. Unknown required qualities remain marked alternatives; contradictions and known ineligibility cannot enter shown exact results. Relaxing a guarantee cannot readmit a mandatory-quality contradiction. Exclusion counts name each listing's primary exclusion cause; its other failed assessments remain in the record.

The fixed examples confirm improvements over the current buyer in handling mandatory contradictions, inferred hints, explicit priorities and individual classifier failures. New noise assessment is an additional capability, not evidence that scoring alone caused every change. The independent real slice shows an order change after removing an orientation hint's advantage. These examples do not establish consumer preference or listing-quality accuracy.

## Final explanations and model value

The [final live run](../../experiments/property-matching/output/live-final.json) used Qwen3.5-9B-4bit, temperature 0 and a 900-token limit through the existing local adapter. It made real baseline writer calls and prototype block-selection calls. All 18 selections validated. The rendered text equals the deterministic fallback exactly; it is backend-authored text, not successful free-form model writing.

| Example | Current generated words | Proposed rendered words |
| --- | ---: | ---: |
| Weighted preferences | 122 | 120 |
| Mandatory-quality alternatives | 145 | 127 |
| Lone result | 106 | 36 |
| Real snapshot listing | 137 | 39 |
| Eligibility before fit | 150 | 84 |
| Direct listing question | 153 | 7 |

Across these 18 fixed examples, median length is 127 versus 68.5 words. Totals are 2,301 versus 1,194 words. Every proposed explanation is shorter than the generated baseline, but reductions vary: the weighted three-listing explanation is only two words shorter because it preserves two actual comparisons and warnings. The current deterministic buyer fallback is usually shorter still, because it gives little ranking information.

Observed median model latency was 7,230.17 ms for the current writer and 1,358.67 ms for block selection. These are single paired observations per fixed case, not a general performance guarantee. Tokens and cost remain unknown. Selecting blocks adds model latency while producing the same text as skipping the call.

The evidence favors rendering explanations directly from backend decisions if the team adopts this style. The retained record and scoring proposal can be assessed independently of that writer choice. Free-form generation failed twice and is not recommended as the authority for reasons. The prototype keeps the constrained call only to make this comparison reviewable.

## Backend operation costs

[Raw benchmarks](../../experiments/property-matching/output/benchmarks.txt) and the [summary](../../experiments/property-matching/output/benchmark-summary.json) contain five repetitions per path on this Mac: Apple M4 Pro, 24 GiB, 12 cores, Go 1.26.4, darwin/arm64. Inference was stopped before timing.

| Fixed workload | Current median µs/op | Proposed median µs/op | Current bytes/op | Proposed bytes/op | Current allocations/op | Proposed allocations/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Weighted preferences | 17.014 | 278.498 | 31,966 | 144,723 | 140 | 1,377 |
| Mandatory alternatives | 18.810 | 335.295 | 33,876 | 171,777 | 161 | 1,696 |
| Shared branch membership | 16.814 | 182.395 | 13,753 | 77,510 | 117 | 953 |
| Real snapshot listing | 9.991 | 93.159 | 6,011 | 46,266 | 64 | 344 |

This prototype is slower and allocates more. It does more work: retained evidence, branch membership, checked JSON sealing, independent projections and fuller explanations. Costs remain below one millisecond at these fixture sizes, but this is not a speed improvement. Larger inventories were not benchmarked. Snapshot loading, SQL, planner/classifier inference and generated writers are outside the timed loops.

## Verification and review

- Focused matching, existing matching adapter, buyer and comparison suites passed during each repair. New behavioral regressions demonstrated failure before repair.
- `go test -json ./... -count=1` passed: 314 test/subtest passes, 29 skips, 23 packages with passing tests and 10 packages without tests. The [raw log](../../experiments/property-matching/output/go-test.jsonl) records skips. PostgreSQL checks did not run against a disposable database.
- The final comparison passes 18 independently labeled fixture orders/completeness expectations. Every shown and excluded score was reconstructed from its retained contributions and denominator.
- Independent Spec review used an oracle frozen before reading proposed code. Standards review covered ownership, legacy callers, input validation, unnecessary abstractions and test value. Verified findings about catalog choices, malformed frozen data, mutation isolation, misleading tradeoff wording, lone fit facts and source scope were repaired and rechecked.
- `gofmt -l` returned no changed Go file. `git diff --check` passed. A diff against baseline confirms buyer, intake, eligibility, HTTP, persistence, provider adapters, frontend, dependency manifests and snapshots unchanged. Existing matching `Evaluate` and adapter tests passed.

The [source manifest](../../experiments/property-matching/output/execution-manifest.json) fingerprints the executed implementation and fixtures. Earlier failed free-writing runs and the pre-review live output remain available; they are not presented as final results.

## Reproduce and assess adoption

Run from the repository root:

```sh
go run ./experiments/property-matching/cmd
python3 docs/experiments/property-matching-verification/run.py
go test ./... -count=1
go test ./experiments/property-matching -run '^$' -bench BenchmarkComparison -benchmem -benchtime=300ms -count=5
```

An optional `-live` run requires an already-running loopback provider and cached model; the experiment never starts or downloads one automatically. See the [README](../../experiments/property-matching/README.md) for the exact flags and boundaries.

The in-memory retrieval adapter proves gates within frozen fixture populations. It does not prove PostgreSQL publication/archiving/paging, full CABA inventory coverage, live criterion extraction or light/noise classifier accuracy. Branch-specific scores are not pooled across different rubrics. The prototype supports `natural_light=high` and `noise_level=quiet`; other values are rejected explicitly. No location proximity behavior exists. No user readability study was run.

Application integration and the separate SQL cleanup remain team decisions. This commit contains the comparison and review artifacts. It does not change the backend request path. No push, PR, merge or deployment occurred.
