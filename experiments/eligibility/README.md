# Eligibility extraction check

`labels.json`: 20 snapshot listings (15 that mention guarantees, 5 that don't), labeled by hand from
their text on 2026-09-23 before any extraction run. Each label records the accepted guarantee
instruments and whether the rule is hard or discretionary. `either` marks ambiguous text and isn't
scored.

| Run | Accepted instruments exact | Hardness |
| --- | --- | --- |
| 1: `garantía` immediately followed by the qualifier | 16/20 | 6/10 |
| 2: qualifier up to a few words later, plus instrument descriptions in the question | 20/20 | 10/10 |

Diagnosis of run 1: three misses were spotting gaps ("garantía **preferentemente** propietaria",
"**garante** fiador propietario", "garante con propiedad"). The fourth ("GARANTIA DE CAPITAL") was
spotted, but Jev didn't know it meant propietaria until the question described the instrument. The
regression test is `TestExtractSpotsPropietariaWrittenOtherWays`.

Caveat: run 2's patterns were fixed using three of these labeled listings, so the set is no longer held
out. Label fresh listings before claiming accuracy.

Full snapshot after run 2: 300 listings examined, 79 with rules. The most common rules are
"propietaria or caución, hard" (42), "caución only, hard" (21) and "propietaria only, hard" (10).
There are 4 income-multiple rules and 6 discretionary rules. Gateway rate limits (429) needed
several resumed runs; the batch command now retries 6 times with backoff starting at 1 s.
