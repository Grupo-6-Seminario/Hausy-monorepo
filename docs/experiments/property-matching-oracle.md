# Property matching oracle

Status: held-out hash frozen 2026-10-04T04:06Z and this file written by 04:09Z, before the verifier read any proposed fit or comparison code. Owner: Claude Opus 5.5 (high) independent verifier, task `task_597c8ff743dc`. Implementation workers must not edit this file.

These cases specify the result the [approved brief](property-matching-prototype.md) requires. Expected values come from the brief, [glossary](../../CONTEXT.md) and evidence text, not from any implementation. Where the brief permits more than one reading, the case lists every accepted result and names the ambiguity. Labels are this verifier's reading of evidence and are frozen assessments for policy checks. They do not measure classifier accuracy.

## Inputs

| Input | Value |
| --- | --- |
| Baseline revision | `7c7012072fc37476a2806b9776771a3bf58d4110` |
| `data/listings.parsed.jsonl` | sha256 `a5c6eeb9e0c2d603daa22130287389dc75087f25d938873ccb43d3a56066630e` |
| `data/listings.eligibility.jsonl` | sha256 `c222088b12a3d818bfd0aca63d58ed7d72141cfcaaef9f39e93a45ddd0b75e54` |
| `data/listings.quality.jsonl` | sha256 `09135de0a5bff359ad6a21e53216f901ec76c0a26fad7a22ad0c75c6f50245d7` |
| Held-out cases | Kept outside the repository; sha256 `aa179eecbbfc2f22790d9131676e4d8b0647c2ef898e9bb6ca8daeba7c55759a`. Revealed in the review after the implementation is final. |

## Policy derived from the brief

1. **Gates.** Neighborhood, operation, currency-qualified price, rooms and other published numeric constraints exclude a listing before scoring. Preference evidence cannot readmit it.
2. **Requirements.** For a mandatory qualitative requirement: supported confirms; contradicted excludes; unknown, hint-only, conflicting and unavailable make an unconfirmed alternative. An alternative is shown after every confirmed listing and is labeled with its state.
3. **Assessments.** Each (listing, criterion) pair has one outcome: `supported`, `contradicted`, `unknown` (no evidence), `hint` (inferred or structural clue only), `conflicting`, or `unavailable` (provider failure). The record keeps the evidence reference and its provenance.
4. **Contribution.** For preferences: supported `+w`, contradicted `-w`, every other outcome `0`. `w` is 1, or 2 only with an explicit priority phrase that appears in the searcher's words.
5. **Score.** `(supported weights - contradicted weights) / sum of all requested preference weights in the listing's branch`. Unknown, hint, conflicting and unavailable preferences stay in the denominator. With no preferences, the score is 0 and the result says there is no preference-based advantage.
6. **Visibility.** Ineligible listings are hidden and counted. Eligible, conditional and unknown eligibility stay visible.
7. **Order.** Confirmation group, then eligibility group (eligible, conditional, unknown), then the explicit sort if any, else descending score, then canonical URL ascending.
8. **Explicit sort.** `price_asc`, `price_desc`, `area_desc`: a missing value sorts last in either direction. The sort replaces the score; equal sort values fall to the URL tie-break.
9. **Deciding reason.** For each consecutive pair, the first rule from step 7 that differs: `confirmation`, `eligibility`, `sort_value`, `sort_missing`, `score` or `tie`. A `tie` claims no property advantage.
10. **Explanation.** Plain Rioplatense Spanish. It cites the deciding reason and source-backed facts, names each condition and unknown once, uses no score terms or percentages, and never states a hint, unknown or unavailable assessment as a fact. A lone result gets a fit explanation without comparison.

Implementations may name outcomes and reasons differently. The review maps names to these meanings. Scores are compared to four decimal places.

## Evidence conventions for synthetic cases

| Label | Evidence and provenance | Expected outcome |
| --- | --- | --- |
| `light+` | Listing claim "muy luminoso" (stated attribute or description) | supported |
| `light-` | Listing claim "poca luz, es oscuro" | contradicted |
| `light~` | Inferred `natural_light=high`, or stated `orientation=norte` / `exposure=frente` only | hint |
| `light!` | "muy luminoso" and "el living es oscuro" in the same ad | conflicting |
| `quiet+` | Listing claim "muy silencioso" | supported |
| `quiet-` | Listing claim "ruidoso, sobre avenida" | contradicted |
| `quiet~` | Inferred `noise_level=quiet` only | hint |
| `balcony+` | Stated `outdoor_space=balcon` | supported |
| `x?` | Nothing about the criterion | unknown |
| `x×` | Classifier or provider error for that listing | unavailable |

All synthetic listings are in Palermo, `alquiler`, ARS 700000, unless the case says otherwise. Qualification is `guarantee=propietaria`. Eligibility rules: `E` = propietaria hard (eligible); `C` = propietaria or caución, discretionary, evidence "ver cuáles permite la propietaria" (conditional); `U` = no rules (unknown); `I3` = income 3x hard with no declared income (unknown); `X` = caución hard (ineligible). URLs are `u/<id>`.

## Part A: assessment labels

These pairs test rubrics and source precedence, not ordering.

| ID | Criterion | Evidence | Expected |
| --- | --- | --- | --- |
| A1 | light | Description "muy luminoso, con sol directo en el living" | supported, ref to that passage |
| A2 | light | Only "dormitorio luminoso" | unknown; the claim covers one room, not the home |
| A3 | light | Stated `orientation=norte`, `exposure=frente`, no light claim | hint, contribution 0 |
| A4 | light | "Ambientes oscuros, poca luz natural" | contradicted |
| A5 | light | "Muy luminoso" and "el living es oscuro" | conflicting, contribution 0 |
| A6 | quiet | Inferred `noise_level=quiet` from "zona residencial" | hint, never supported |
| A7 | quiet | Inferred `noise_level=moderate` | hint or unknown, never contradicted |
| A8 | balcony | Stated `outdoor_space=balcon` and inferred `outdoor_space=none` | supported; the reference is the stated attribute |
| A9 | balcony | Inferred `outdoor_space=balcon` only | hint |
| A10 | quiet | Neighborhood reputation only ("Palermo es tranquilo") | unknown or hint, never supported |
| A11 | light | Provider error | unavailable; distinct from unknown; never "el aviso no lo menciona" |

## Part B: policy cases

### B1. Gates

Search: Palermo, alquiler, ARS, max 800000, rooms 2..2. Preference: light w1.

| ID | Variation | Light | Expected |
| --- | --- | --- | --- |
| g-ok | 700000, 2 rooms | `light?` | shown, score 0 |
| g-price | 850000 | `light+` | excluded |
| g-usd | USD 700 | `light+` | excluded |
| g-rooms | 3 rooms | `light+` | excluded |
| g-unpriced | no published price | `light+` | excluded (price bound without `include_unpriced`) |
| g-hood | Belgrano, 600000 | `light+` | excluded |

Shown: `g-ok`. It is a lone result: fit explanation, no comparison. Both baseline and proposed paths must apply the same gate. If the comparison delegates gates to SQL and feeds pre-filtered inventory, the report must say so; a neighborhood-only stub does not prove price or room gates.

### B2. Nearby wrong-neighborhood control

Search: Palermo only. Preferences: light w1, quiet w1.

| ID | Hood | Light | Quiet |
| --- | --- | --- | --- |
| n-in | palermo | `light?` | `quiet?` |
| n-near | colegiales | `light+` | `quiet+` |

Shown: `n-in` (score 0). `n-near` is absent from results, ranks and the explanation. The explanation does not offer it as a nearby suggestion.

### B3. Contributions, equal weights

Preferences: light w1, quiet w1. All `E`.

| ID | Light | Quiet | Score |
| --- | --- | --- | --- |
| p-both | `light+` | `quiet+` | 1.0000 |
| p-light | `light+` | `quiet?` | 0.5000 |
| p-hint | `light~` | `quiet~` | 0.0000 |
| p-none | `light?` | `quiet?` | 0.0000 |
| p-dark | `light-` | `quiet+` | 0.0000 |
| p-noisy | `light?` | `quiet-` | -0.5000 |

Order: `p-both, p-light, p-dark, p-hint, p-none, p-noisy`.
Reasons: score, score, tie, tie, score.
`p-dark` before `p-hint` is a URL tie: the explanation must not prefer it. `p-hint` may mention "orientación norte" only as an indication, not as "luminoso".

Observed current buyer: `p-both, p-light, p-hint, p-dark, p-noisy, p-none`. The baseline counts inferred attributes and the orientation clue as fit and ignores contradictions.

### B4. Explicit priority

Message: "Busco en Palermo, luminoso, y sobre todo que sea tranquilo". Preferences: light w1 (no priority phrase); quiet w2, source "sobre todo que sea tranquilo". All `E`. Denominator 3.

| ID | Light | Quiet | Score |
| --- | --- | --- | --- |
| q-both | `light+` | `quiet+` | 1.0000 |
| q-quiet | `light?` | `quiet+` | 0.6667 |
| q-light | `light+` | `quiet?` | 0.3333 |
| q-lightnoisy | `light+` | `quiet-` | -0.3333 |

Order: `q-both, q-quiet, q-light, q-lightnoisy`. Reasons: score, score, score.
Counterfactual without the priority (both w1): `q-both 1, q-light 0.5, q-quiet 0.5, q-lightnoisy 0` and order `q-both, q-light, q-quiet, q-lightnoisy` (`q-light`/`q-quiet` is a URL tie).

Observed current buyer: `q-both, q-light, q-lightnoisy, q-quiet`. It has no priorities and places the listing that contradicts the prioritized quality above one that supports it.

### B4b. Invalid priority

| Variant | Expected |
| --- | --- |
| quiet w2 with source "es fundamental el silencio", absent from the message | Rejected. Must never score with w2. Silent fallback to w1 is a lesser defect. |
| quiet w3 | Rejected. |
| light w2 inferred from criterion type or classifier confidence, no phrase | Rejected or w1; never w2. |

### B5. Mandatory requirement and alternatives

Requirement: light, "tiene que ser luminoso". Preference: quiet w1. All `E`.

| ID | Light (required) | Quiet | Group | Score |
| --- | --- | --- | --- | --- |
| m-sup-quiet | `light+` | `quiet+` | confirmed | 1 |
| m-sup | `light+` | `quiet?` | confirmed | 0 |
| m-unk | `light?` | `quiet+` | unconfirmed | 1 |
| m-conflict | `light!` | `quiet?` | unconfirmed (conflicting) | 0 |
| m-hint | `light~` | `quiet?` | unconfirmed (hint) | 0 |
| m-scope | "dormitorio luminoso" only (A2) | `quiet?` | unconfirmed | 0 |
| m-contra | `light-` | `quiet+` | excluded | — |

Order: `m-sup-quiet, m-sup, m-unk, m-conflict, m-hint, m-scope`. Reasons: score, confirmation, score, tie, tie.
The explanation never calls `m-unk`, `m-conflict`, `m-hint` or `m-scope` luminoso. It distinguishes "no lo menciona" (`m-unk`), "información contradictoria" (`m-conflict`) and "hay indicios" (`m-hint`).

Observed current buyer: `m-sup-quiet, m-sup, m-contra, m-hint, m-unk, m-conflict, m-scope`. The baseline shows a mandatory contradiction as an unconfirmed alternative and ranks it first among them.

Accepted alternative for `m-conflict`: exclusion, if the implementation documents conflicting mandatory evidence as failing. Showing it as confirmed is a failure.

### B6. Eligibility groups

Preference: light w1.

| ID | Rule | State | Light | Score |
| --- | --- | --- | --- | --- |
| e-elig | E | eligible | `light?` | 0 |
| e-cond | C | conditional | `light+` | 1 |
| e-unk | U | unknown | `light+` | 1 |
| e-unk2 | I3 | unknown (missing income) | `light?` | 0 |
| e-inel | X | ineligible | `light+` | hidden |

Order: `e-elig, e-cond, e-unk, e-unk2`. Reasons: eligibility, eligibility, score. Hidden count 1; relaxation: caución returns 1.
The explanation names `e-cond`'s condition "ver cuáles permite la propietaria" once and says `e-unk`/`e-unk2` eligibility is unconfirmed. It does not imply `e-elig` fits better.

Observed current buyer: identical order and hidden count.

### B7. Branch-specific preferences and an empty branch

Branch A: Palermo, preferences light w1, quiet w1. Branch B: Caballito, preference balcony w1. Branch C: Belgrano, no candidates. All `E`.

| ID | Branch | Assessments | Score |
| --- | --- | --- | --- |
| a1 | A | `light+`, `quiet?` | 0.5000 (of 2) |
| a2 | A | `light+`, `quiet+` | 1.0000 (of 2) |
| b1 | B | `balcony+` | 1.0000 (of 1) |
| b2 | B | `balcony?` | 0.0000 (of 1) |

Order (pooled list): `a2, b1, a1, b2`. Reasons: tie, score, score.
`b1` carries no light or quiet assessment; `a1`, `a2` carry no balcony assessment. Branch C is reported with 0 results. The explanation compares coverage of each branch's own request and never claims `b1` is more luminous than `a1`.
Ambiguity: the brief does not choose pooled or per-branch presentation. A per-branch result `A: a2, a1; B: b1, b2; C: empty` is accepted if the report says so.

### B8. Explicit sorts and missing values

B8a, `area_desc`, preference light w1, all `E`:

| ID | Area m² | Light | Score |
| --- | --- | --- | --- |
| s-big | 80 | `light?` | 0 |
| s-big-tie | 80 | `light+` | 1 |
| s-small | 40 | `light+` | 1 |
| s-noarea-a | missing | `light+` | 1 |
| s-noarea-b | missing | `light?` | 0 |

Order: `s-big, s-big-tie, s-small, s-noarea-a, s-noarea-b`. Reasons: tie, sort_value, sort_missing, tie.
Ambiguity: if the implementation breaks equal sort values by score, `s-big-tie, s-big` is accepted only when the report documents it; the brief says the sort replaces the score. In either case the `s-big`/`s-big-tie` pair claims no fit advantage unless score actually decided it.

B8b, `price_desc`, no preferences: `d-unk-exp` (U, 1000000), `d-elig-cheap` (E, 600000), `d-elig-exp` (E, 900000). Order: `d-elig-exp, d-elig-cheap, d-unk-exp`. Reasons: sort_value, eligibility.

B8c, `price_asc`, requirement light: `x-unconf` (`light?`, 400000), `x-conf` (`light+`, 900000). Order: `x-conf, x-unconf`. Reason: confirmation.

Observed current buyer for B8a to B8c: identical orders.

### B9. No preferences

Three `E` listings `n-b` (700000), `n-c` (600000), `n-a` (800000); no preferences, default sort.
Order: `n-a, n-b, n-c` by URL. All scores 0 with the no-preference indication. Reasons: tie, tie. The explanation contains no "mejor", "más conveniente" or fit comparison.

### B10. Lone result

One `E` listing `l-1`, preference light w1, `light+`. Score 1. No consecutive reasons. The explanation states why it fits and contains no comparative ranking language ("queda arriba", "a diferencia de", "la primera ... la segunda").

### B11. Empty results

B11a: Palermo search with no candidates. No ranks, branch count 0; the explanation says nothing was found and invents no listing.
B11b: two listings, both `X`. Shown empty; hidden 2; relaxation caución returns 2. The explanation reports the hidden count and the relaxation once.

### B12. Provider failure

Requirement: light. Preference: quiet w1. All `E`. The classifier fails for `f-fail` only.

| ID | Light | Quiet | Group | Score |
| --- | --- | --- | --- | --- |
| f-ok | `light+` | `quiet?` | confirmed | 0 |
| f-fail | `light×` | `quiet×` | unconfirmed (unavailable) | 0, quiet stays in denominator |

Order: `f-ok, f-fail`. Reason: confirmation. `f-fail` is not excluded and not contradicted; the explanation says the check was not possible and never says the ad omits light. The failure of `f-fail` does not change `f-ok`'s assessments.
If every classifier call fails, every listing is an unconfirmed alternative, still ordered by eligibility then URL.

Observed current buyer: `f-fail, f-ok`, both unconfirmed. One failing listing aborts its whole batch, so `f-ok` loses its confirmation.

### B13. Source precedence

Preference: balcony w1. All `E`.

| ID | Evidence | Outcome | Score |
| --- | --- | --- | --- |
| pr-stated | Stated `outdoor_space=balcon`; inferred `outdoor_space=none` | supported, stated reference | 1 |
| pr-inferred | Inferred `outdoor_space=balcon` only | hint | 0 |

Order: `pr-stated, pr-inferred`. Reason: score.

## Part C: real snapshot case R1

Search: palermo, alquiler, ARS, max 700000, rooms 2..2. Qualification `guarantee=propietaria`. Preferences: light w1 ("luminoso"); quiet w2 ("sobre todo tranquilo"). Denominator 3. Listings are identified by their URL's numeric suffix.

| Listing | Eligibility | Light | Quiet | Score |
| --- | --- | --- | --- | --- |
| 59791377 | ineligible (caución only) | supported, "luminoso" | hint | hidden |
| 59889701 | eligible | supported, "ventanales en ambos ambientes que aseguran gran luminosidad" | hint (inferred from "Muy Bajas expensas") | 0.3333 |
| 60033803 | eligible | unknown | hint (inferred from "Zona de cafecitos") | 0.0000 |
| 59918611 | unknown (no rules) | supported from description "muy luminoso, excelente luminosidad natural"; the parsed attribute is marked inferred | ambiguous: "entorno residencial y tranquilo" | 1.0000 if supported, 0.3333 if unknown |

~~Order under either quiet label: `59889701, 60033803, 59918611`.~~ Superseded by amendment 1.
Fails if: an inferred quiet attribute counts as support, a listing appears outside its eligibility group, or 59791377 appears.

**Amendment 1 (2026-10-04, written between 04:09Z and 04:15Z, after reading the implementation's loader).** The frozen table missed two inputs the application applies:

- 59889701 has no record in `data/listings.quality.jsonl`. Retrieval requires `quality_status IN ('legacy','passed')` (`internal/store/postgres/search.go:51`), so it is withheld from buyer search.
- 60033803 states `pets_allowed=no` ("No se aceptan mascotas"). The application loader adds `eligibility.PublishedRules` (`internal/pipeline/eligibility.go:96`), so it carries a pets rule. The searcher declared no pets fact, so its eligibility is unknown, not eligible.

Corrected R1: `59918611, 60033803`, both unknown eligibility. Reason: score (1.0000 or 0.3333 against 0.0000). Hidden 1 (59791377); relaxation caución returns 1. The same corrections apply to held-out H2: 59889701 is withheld and 59775328 is unknown (pets rule).

The baseline observation below fed all four rows to the fake inventory without the publication gate or derived pets rules. It is kept as recorded, but it is not a valid R1 baseline.

Observed current buyer, with and without a classifier: `59889701, 60033803, 59918611`, hidden 1.

## Explanation checks

Applied to every generated or fallback explanation:

- Every rank reference matches the recorded order.
- Each consecutive comparison cites the recorded deciding reason. A `tie` pair gets no comparative claim.
- Each positive quality claim has a supported assessment with a source reference.
- Each shown eligibility condition and unknown appears once, next to its listing.
- No score, percentage, probability or rubric term reaches user prose.
- Unavailable is never phrased as omission; hint is never phrased as fact.
- At most three explained listings, one short overview and two or three short sentences each.

## Reproducing the baseline observations

The verifier ran the current buyer (`buyer.NewAgent`, real `rank`) on B3, B4, B5, B6, B8, B9, B12 and R1 through a test file injected with `go test -overlay`; no repository file changed. Gates were applied before the fake inventory for R1. Command:

```sh
go test -overlay <scratch>/baseline/overlay.json ./internal/buyer -run 'TestOracleBaseline' -v -count=1
```

The harness is kept outside the repository; sha256 `a5384c684fe3295ec45ad7e924a38e471290c46e252f9d25bd5f5c32a45fce27`. The baseline uses frozen light answers for the classifier; it sends no other criterion to the classifier.

## Amendment 2: targeted listing questions

Added 2026-10-04 between 04:20Z and 04:25Z at the coordinator's request, after the verifier had read the implementation. These cases follow the brief ("A targeted listing question is answered directly without a ranking recap") and were not tuned to observed output.

Search: Palermo; requirement light; preference quiet w1. Listings: `t-ok` (`light+`, `quiet?`, E), `t-fail` (`light×`, `quiet×`, E), `t-hint` (`light~`, `quiet-`, C), `t-noprice` (no price, `light+`, E), and `t-near` (Colegiales, gated out).

| Question | Expected answer content | Must not contain |
| --- | --- | --- |
| `t-ok` light | the ad states it | "no confirma" |
| `t-ok` quiet | the ad does not confirm it | a support claim |
| `t-fail` light or quiet | the check was unavailable | "no menciona" or any omission claim |
| `t-hint` light | indications only, unconfirmed | a support claim |
| `t-hint` quiet | the ad contradicts it | a support claim |
| `t-hint` eligibility | "ver cuáles permite la propietaria" | "podés alquilar" |
| `t-noprice` price | the ad does not publish a price | a numeric price |
| `t-near` anything | no data for that listing in this search | any fact about it |

No answer contains a rank number or ordering sentence.
