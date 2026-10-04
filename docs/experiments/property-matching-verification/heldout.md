# Held-out property matching cases

Verifier-owned. Frozen before the implementation was read. Revealed in the review only after the implementation is final.

Conventions are those of docs/experiments/property-matching-oracle.md: score = (supported weights - contradicted weights) / all requested preference weights; order = confirmation group, eligibility group (eligible, conditional, unknown), then explicit sort or descending score, then canonical URL ascending.

## H1. Combined gates, requirement, priority, eligibility, sort and provider failure

Search: Palermo, alquiler, no price bound, sort price_asc. Qualification: guarantee=propietaria.
Requirement: natural light, "tiene que ser luminoso". Preferences: quiet weight 2, source "sobre todo silencioso"; balcony weight 1, no source.

| ID (URL u/...) | Hood | Price ARS | Eligibility rule | Light (required) | Quiet (w2) | Balcony (w1) |
| --- | --- | --- | --- | --- | --- | --- |
| h1-a | palermo | 800000 | propietaria hard | supported | unknown | supported |
| h1-b | palermo | 500000 | propietaria|caucion discretionary | supported | unknown | unknown |
| h1-c | palermo | missing | propietaria hard | supported | supported | supported |
| h1-d | palermo | 600000 | propietaria hard | supported | contradicted | unknown |
| h1-e | palermo | 300000 | propietaria hard | unavailable (provider failure, all criteria) | unavailable | unavailable |
| h1-f | palermo | 200000 | propietaria hard | contradicted | supported | supported |
| h1-g | palermo | 250000 | caucion hard | hint only | unknown | unknown |
| h1-h | belgrano | 100000 | propietaria hard | supported | supported | supported |

Expected shown order: h1-d, h1-a, h1-c, h1-b, h1-e.
Groups: h1-d, h1-a, h1-c confirmed/eligible; h1-b confirmed/conditional; h1-e unconfirmed/eligible (status unavailable, not contradicted, not "no lo menciona").
Excluded: h1-f (mandatory contradiction), h1-h (neighborhood gate). Hidden ineligible: h1-g (count 1).
Scores (denominator 3): h1-a 1/3, h1-b 0, h1-c 1, h1-d -2/3, h1-e 0.
Deciding reasons: d->a price value; a->c missing price sorts last; c->b eligibility; b->e confirmation.
Explanation: h1-d first because of price, never because of fit; its quiet contradiction is not presented as support.

## H2. Real snapshot slice, larger

Search: palermo, alquiler, ARS, max 1,000,000, rooms 2..2. Qualification guarantee=propietaria. Preferences: light weight 1 ("luminoso"); quiet weight 2 ("sobre todo tranquilo"). Snapshot hashes as in the oracle.

Labels (listing ID suffix):

| ID | Eligibility | Light | Quiet |
| --- | --- | --- | --- |
| 59889701 | eligible | supported, "ventanales ... gran luminosidad" | hint (inferred from "Muy Bajas expensas") |
| 60033803 | eligible | unknown | hint (inferred) |
| 59775328 | eligible | AMBIGUOUS: supported ("sol de mañana") or unknown | hint (inferred "Vista abierta al pulmón de manzana") |
| 59918611 | unknown (no rules) | supported from prose "muy luminoso, excelente luminosidad natural" (attribute marked inferred) | AMBIGUOUS: supported ("entorno residencial y tranquilo") or unknown |
| 59835247 | unknown | supported "Muy buena luz natural" | hint |
| 59679253 | unknown | supported "disfruta de abundante luz natural" | hint (moderate, inferred) — not a contradiction |
| 59906462 | unknown | unknown (orientation norte inferred is a hint) | hint (moderate, inferred) — not a contradiction |
| 59470722 | unknown | unknown | hint (inferred from "SEGURIDAD 24HS") |
| 59926573, 59881443, 59791377, 59602413 | ineligible (caución only) | — | — |

Expected order, light(59775328)=supported, quiet(59918611)=supported:
59775328 (1/3), 59889701 (1/3), 60033803 (0), 59918611 (1), 59835247 (1/3), 59679253 (1/3), 59470722 (0), 59906462 (0). Hidden 4.

Accepted alternatives under the ambiguous labels:
- 59775328 light unknown: eligible group becomes 59889701, 59775328, 60033803.
- 59918611 quiet unknown: unknown group becomes 59835247, 59679253, 59918611, 59470722, 59906462.

Any ordering of the eligible group after the unknown group, any inferred quiet counted as support, or any inferred "moderate" counted as contradiction is a failure.

## H3. Branch-specific preferences and partial provider failure

Branch A: Palermo, preference light weight 1. Branch B: Caballito, no preferences. Qualification guarantee=propietaria; all candidates propietaria hard (eligible).

| ID | Branch | Light |
| --- | --- | --- |
| a-sup | A | supported |
| a-fail | A | unavailable (provider failure for this candidate only) |
| b-1 | B | not assessed (criterion not in branch B) |

Expected order: a-sup (1), a-fail (0), b-1 (0, no preference-based advantage). Reasons: score; tie.
b-1 carries no light assessment. a-fail keeps light in its denominator with status unavailable. a-sup keeps its supported status despite a-fail's failure (no batch contamination).
Explanation: never says b-1 fits worse or lacks light; never says a-fail's ad omits light.
