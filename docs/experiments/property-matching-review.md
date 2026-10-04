# Property matching independent review

Reviewer: Claude Opus 5.5 (`claude-opus-5-5`), high effort, Orca task `task_597c8ff743dc`, dispatch `ctx_1dad8a1787d0`. The model ID comes from the session's system identity. The effort comes from the launch receipt recorded in [results](property-matching-results.md).

Scope: Spec review of the implementation against the [approved brief](property-matching-prototype.md) and the independent [oracle](property-matching-oracle.md), plus a Standards pass. The reviewer wrote no implementation file. Every executable check was injected with `go test -overlay`.

Reviewed code: `internal/matching/fit.go`, `internal/matching/fit_test.go` and `experiments/property-matching/`, at the revised freeze of 2026-10-04T04:44Z. Its source hashes match `output/execution-manifest.json` (file sha256 `68c66f921af0da71371952e08e732777ad71cfea69b027bd34fd5056b6e73b7a`; sources re-verified at 04:50Z). An earlier freeze (04:36Z, `output/execution-manifest-before-review-fixes.json`) was also fully tested. The two differ only in `explanation.go` and `comparison_test.go`.

## Verdict

The ordering policy matches the oracle on every case. All 35 independent controls pass on the revised code. They cover:

- gates and the nearby-neighborhood control;
- contributions, explicit priorities and invalid priorities;
- mandatory alternatives and the four eligibility states;
- branches, sorts with missing values, ties and no preferences;
- lone, empty and all-ineligible results;
- provider failure and source precedence;
- targeted questions, relaxations, real snapshot slices and the three held-out cases.

Every shown score equals its numerator over its denominator, and the contributions sum to the numerator.

The explanation findings S1, S3 and S5 were repaired after the first freeze and re-verified. The language model adds no content. The closed writer can only echo block IDs, so the rendered text is always the deterministic text (S2). Two earlier free-writing attempts with the local model changed facts. The deterministic path is the defensible one to adopt.

The proposed path costs 9–18 times the baseline's time and 5–11 times its allocations per offline operation. It stays under 0.34 ms. The work compared is not equal (T5).

## Oracle results

| Case | Proposed result | Oracle | Current buyer (observed) |
| --- | --- | --- | --- |
| B1 gates | `g-ok` only | pass | `g-ok` only (same in-memory gate) |
| B2 nearby | `n-in` only; Colegiales absent from results and prose | pass | `n-in` |
| B3 contributions | `p-both, p-light, p-dark, p-hint, p-none, p-noisy`; scores 1, .5, 0, 0, 0, -.5 | pass | `p-both, p-hint, p-dark, p-light, p-noisy, p-none`: hints count, contradictions ignored |
| B4 priority | `q-both, q-quiet, q-light, q-lightnoisy`; .6667, .3333, -.3333 | pass | `q-both, q-light, q-lightnoisy, q-quiet`: a listing that contradicts the prioritized quality ranks above one that supports it |
| B4b invalid priority | phrase absent from the question, weight 3, no phrase: all rejected | pass | n/a |
| B5 mandatory | `m-sup-quiet, m-sup, m-unk, m-conflict, m-hint, m-scope`; `m-contra` excluded | pass | `m-contra` shown as the first alternative |
| B6 eligibility | `e-elig, e-cond, e-unk, e-unk2`; hidden 1; caución returns 1 | pass | same |
| B7 branches | A: `a2, a1`; B: `b1, b2`; C: empty and reported | pass (per-branch, accepted by the coordinator) | pooled `a2, a1, b1, b2` |
| B8a area | `s-big, s-big-tie, s-small, s-noarea-a, s-noarea-b`; equal area is a stable tie | pass | same |
| B8b, B8c price | `d-elig-exp, d-elig-cheap, d-unk-exp`; `x-conf, x-unconf` | pass | B8c: `x-unconf, x-conf` |
| B9, B10, B11 | URL order with ties; lone fit explanation; empty and all-ineligible reported | pass | same order |
| B12 provider failure | `f-ok` confirmed; `f-fail` unconfirmed and unavailable | pass | `f-fail, f-ok`, both unconfirmed: one failure demotes the whole batch |
| B13 precedence | stated balcony supported; inferred-only balcony is a hint | pass | `pr-inferred, pr-stated` |
| R1 real slice | `59918611, 60033803`, both unknown eligibility; hidden 1 | pass after oracle amendment 1 | `59918611, 60033803` |
| H1 held-out | `h1-d, h1-a, h1-c, h1-b, h1-e`; `h1-f` excluded, `h1-g` hidden, `h1-h` gated out | pass | `h1-f` (mandatory contradiction) ranked first |
| H2 held-out | `59775328, 59835247, 59679253, 59918611, 59470722, 60033803, 59906462`; hidden 4 | pass after amendment 1 | `59775328, 59679253, 59835247, 59470722, 59918611, 60033803, 59906462`: orientation hints move 59679253 up |
| H3 held-out | A: `a-sup, a-fail`; B: `b-1`; `b-1` has no light assessment | pass | pooled; `a-sup` keeps its support |
| Targeted questions (9) | direct answers; unavailable never phrased as omission; no rank recap | pass | n/a |
| Pets "none" probe | 60033803 eligible for a searcher with no pets | pass (fixed during review) | n/a |
| Relaxation gates | caución promises 1, not counting a contradicted or gated listing | pass | n/a |

The current-buyer column has two sources:

- the reviewer's overlay harness on the unmodified buyer;
- for B1–H3, `comparison.Run` with `ORACLE_BASELINE=1`, which runs `buyer.NewAgent` on the experiment's gated inventory.

B3's baseline order differs between the two sources (`p-hint` second versus third) because the experiment derives evidence differently. Both sources show the same defects.

### Corrections to the reviewer's own artifacts

- **Oracle amendment 1.** Two inputs were missing from the frozen R1 table. 59889701 has no quality review, so the publication gate withholds it. 60033803 and 59775328 carry a pets rule that the loader derives. The implementation's loader exposed both errors. The held-out file keeps its frozen hash; its H2 expectations were corrected the same way at evaluation time.
- **Oracle amendment 2** (targeted questions) was written after reading the implementation, at the coordinator's request. It is not independent of the implementation.
- **Lexical assertion.** After the wording repair, one targeted control required the literal substring "ruido". The new answer is "La información del aviso contradice tu preferencia de silencio." That meets amendment 2, which asks for contradiction content and no support claim. The control now requires "contradice" and forbids "silencioso". The preserved original is `oracle-controls.go.txt` (sha256 `c685f712…26d6ad`). The corrected control is sha256 `55cc8e54…246be6`. This one line is the only change.

## Spec findings

| ID | Severity | Finding | Evidence | Disposition |
| --- | --- | --- | --- | --- |
| S1 | Medium | A score comparison presented adverse facts as reasons after "porque". Before the fix, B4 rank 2 read: "Queda antes de #3 porque el aviso no confirma luminoso; …". | `explanation.go` `orderingText`; `independent-before-wording-fixes.txt` | Fixed and verified. The text now reads "porque priorizaste tranquilo y el aviso describe el departamento como silencioso; en #3, el aviso no confirma tranquilo. A cambio, el aviso no confirma luminoso; …". B3 rank 2 also puts its adverse fact after "A cambio". |
| S2 | Medium (scope) | The closed writer asks the model to return every block ID in order, and rendering always uses the backend blocks. In `live-final.json`, all 18 cases have status `validated_blocks` and the rendered text equals the fallback. The model adds latency and no content. The brief's "real generated prose is reviewed" criterion is therefore answered as "no model-authored prose", not as a passing review of generated prose. | `explanation.go` `GeneratePair`; `output/live-final.json` | Accepted by the coordinator. Recommend adopting the deterministic path and dropping the model call. |
| S3 | Low | Every quiet contradiction rendered as "el aviso menciona ruido dentro del departamento", whatever the evidence said. | `criterionPhrase` before the fix | Fixed and verified: "La información del aviso contradice tu preferencia de silencio." |
| S4 | Low | A listing that is both ineligible and contradicts a mandatory quality is counted only under "excluida por los requisitos para alquilar". | Overlay `TestOracleRelaxationGates`: "Hay 2 opciones excluidas por los requisitos para alquilar. Si conseguís una caución, vuelve una propiedad." | Documented as a policy: the primary cause is counted, both assessments are retained, and the relaxation count stays correct (1 of 2). The searcher still does not learn about the contradiction. Acceptable for the prototype; revisit at integration. |
| S5 | Low | A lone result in a requirement-only search read "No pediste preferencias…" and did not say the requirement was confirmed. | Overlay relaxation case before the fix | Fixed and verified: "Cumple los filtros publicados y el aviso describe la propiedad como luminosa." |
| S6 | Info | The committed fixtures hold one real row, so they show no real ordering change. The reviewer's R1 and H2 slices add seven real Palermo rows and one real order change caused by hints (H2). These slices are not in the command's report. | `fixtures/cases.json`; overlay `TestOracleReal` | Open; adding a real multi-row slice would strengthen the comparison. |

Fixed earlier in the review and re-verified on the revised code:

- The pets catalog had no choices, so searchers declaring "no tengo mascotas" became ineligible.
- Fixture mistakes silently became `provider_error`.
- Lone and last rows had no fit facts.
- Prose defects: "1 opciones", the raw `caucion` slug, and a relaxation sentence attached to the contradiction note.
- Duplicate `hint` evidence IDs.

The free-writer inversion of quiet evidence cannot recur, because model text no longer reaches the output.

## Live prose observed

All runs used local Qwen3.5-9B-4bit through loopback. No paid calls were made. Token and cost telemetry were not observed. They are not zero.

| Run | Proposed result | Assessment |
| --- | --- | --- |
| `live-comparison.json` (free writer v1) | mandatory-alternatives: "cumple con la preferencia de tener mucho ruido de tránsito" for #1, whose quiet is supported. weighted-net: "perdiendo puntos", and "cae al final" when five results exist. lone: "el sistema lo clasifica como confirmado", "en este paquete". | Fail: inverted evidence, score jargon, wrong position, an invented comparison for a lone result |
| `live-comparison-v2.json` (Spanish facts input) | "El aviso contradice silencio, así que no es ruidoso"; "garantiza silencio"; "El ranking está completo" | Fail: inverted evidence, overclaim, internal field leak |
| `live-final.json` (closed writer, revised code) | 18 of 18 `validated_blocks`; text identical to the fallback | Faithful by construction; no model value |

From `live-final.json`:

- Proposed text is shorter than the generated baseline in all 18 cases: 1194 against 2301 words in total, median 68.5 against 127.
- The proposed call takes a median of about 1.4 s against about 7.2 s for the baseline writer. All of that 1.4 s is spent echoing IDs.

Baseline live prose has its own errors. In weighted-net it claims that every listing has "buena luz y tranquilidad", although one listing is documented as dark and noisy. In mandatory-alternatives it says "siete opciones" when five were shown.

## Benchmarks

Source: `output/benchmarks.txt` (sha256 `40bed249…88e5cc4`), Apple M4 Pro, go1.26.4, `-benchtime=300ms -count=5`, local inference stopped. Medians of five runs:

| Case | Baseline ns/op | Proposed ns/op | Ratio | Baseline B/op, allocs | Proposed B/op, allocs |
| --- | --- | --- | --- | --- | --- |
| weighted-net | 17,014 | 278,498 | 16.4× | 31,966, 140 | 144,723, 1377 |
| mandatory-alternatives | 18,810 | 335,295 | 17.8× | 33,876, 161 | 171,777, 1696 |
| branch-specific-shared | 16,814 | 182,395 | 10.8× | 13,753, 117 | 77,510, 953 |
| snapshot-direct-light | 9,991 | 93,159 | 9.3× | 6,011, 64 | 46,266, 344 |

The proposed timing includes work the baseline does not do: sealing the record, JSON projection copies, branch records and fallback rendering. The baseline timing includes the buyer's writer-error fallback. Neither includes SQL, a classifier or a model. At under 0.34 ms per operation, the difference is negligible next to a model call measured in seconds. It is still a real compute regression, and nothing here shows a speed benefit. A likely contributor is the JSON round trip in `Record.Project()`, which `Fallback` reaches more than once. This was not profiled.

## Standards findings

| ID | Finding | Location | Disposition |
| --- | --- | --- | --- |
| T1 | Priority validation is split across two checks. `AssessFit` checks `PrioritySource` against the caller's `SourceQuote`; `RunProposed` checks it against `Case.Question`. | `fit.go`; `comparison.go` | The coordinator states the two layers guard distinct trusts. Agreed for the prototype. At integration, one owner must check against the searcher's actual messages. |
| T2 | `FitCriterion` embeds `Criterion.Priority` (`primary`/`secondary`), and `AssessFit` ignores it in favor of `Weight`. | `fit.go` | Documented as a prototype choice. Leaves two priority vocabularies on one type until integration. |
| T3 | ~~Preflight runs only in `Load`.~~ Withdrawn: `RunProposed` calls `validateFrozen` (`comparison.go:189`). This was the reviewer's misreading. | — | Retracted |
| T4 | `TestFrozenDevelopmentCases` checks shown order only. Scores and reasons per fixture case are asserted in `fit_test.go`, the named comparison tests and these independent controls. | `comparison_test.go` | Accepted as complementary coverage |
| T5 | The benchmark compares unequal work; see [Benchmarks](#benchmarks). | `benchmark_test.go`, README | Accepted; the README says the same |

Checks run by the reviewer on the frozen code:

- `go test ./internal/matching/ ./experiments/property-matching/... ./internal/buyer/ -count=1`: ok.
- `go vet` on those packages: clean.
- `gofmt -l`: empty.
- `git diff --check`: clean.

The parent reports `go test -json ./... -count=1`: 314 passed, 29 skipped. The reviewer did not rerun the full suite.

## Limitations

- Assessments are frozen labels by this reviewer. Nothing here measures classifier accuracy, Jev behavior, or the new light and noise rubrics on live text.
- The in-memory gate mirrors the SQL predicates as read from the code. No PostgreSQL, paging or archive behavior was exercised.
- Per-branch presentation was accepted as instructed. A pooled cross-branch ordering was not evaluated.
- Explanation checks are this reviewer's string and fact checks against the retained records. No readability study with users was done.

## Reproduction

The parent preserved the reviewer's artifacts in [property-matching-verification/](property-matching-verification/):

| File | sha256 | Content |
| --- | --- | --- |
| `oracle-controls.go.txt` | `c685f712fce4efa42e850223ca77fe7530299c2badcad1f2a28243914e78d6ad` | 35 controls as run at the 04:36Z freeze (before the one-line lexical fix described above) |
| `baseline-controls.go.txt` | `a5384c684fe3295ec45ad7e924a38e471290c46e252f9d25bd5f5c32a45fce27` | Current-buyer observation harness |
| `heldout.md` | `aa179eecbbfc2f22790d9131676e4d8b0647c2ef898e9bb6ca8daeba7c55759a` | Held-out cases, frozen before the implementation was read |
| `independent-before-wording-fixes.txt` | `45dbc5bd3e9e8ed87a391afcb1c02e3e660e117a84f73586434b5f19aa47778c` | Raw control output at the 04:36Z freeze |
| `independent-after-wording-before-lexical-fix.txt` | `044cab84627d1483c930f71fcf6cf0ac3bbc2d442d75ca39e50511912391926f` | Parent's rerun before the lexical assertion correction, with its failure preserved |
| `oracle-controls-final.go.txt` | `55cc8e54f6d70e85653cc4b309c05d954cd17751aa788d12a0149c15ee246be6` | Final controls, with only the disclosed lexical assertion correction |
| `independent-final.txt` | `9e193e3e11363c0cb9884ab94803518836767b94984a29ac0a2bb3ba42fdc446` | Parent's portable final rerun: all controls and baseline observation harness pass |

Run from the repository root:

```sh
python3 docs/experiments/property-matching-verification/run.py
```

`run.py` resolves the repository root from its own location and writes a temporary overlay. It adds no files to application packages. The runner uses `oracle-controls-final.go.txt`. Every control passes against the revised code. Its only change from the frozen original is the disclosed lexical `t-hint`/noise assertion correction. Both the original harness and the earlier failed rerun are preserved. The parent executed this portable runner successfully after copying the corrected harness.
