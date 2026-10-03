# Eligibility rules are data, evaluated in code; models only extract them at load time

A listing's eligibility requirements are extracted once when listings are loaded. Go finds the text and
numbers, and Jev classifies each guarantee instrument as accepted, rejected or not stated, and each rule as
hard or discretionary. The output is committed as JSONL and stored as rule rows (fact, operator, value,
hardness, visibility, source). At search time, a pure Go function evaluates those rows against the
searcher's qualification and returns one of the four eligibility states with the named condition and
evidence. We chose this over asking a model at search time because eligibility is the product's core
promise and must be deterministic, auditable and fast. `idea.md` §8 forbids hard-coding guarantee
instruments. Keeping rules and facts as rows lets published, declared, observed and (later) private agency
rules share one evaluator without a schema rewrite before the 2026-11-01 freeze.

## Consequences

- Rules may only reference facts marked admissible in a data table. Protected characteristics (age,
  nationality, gender, …) are not admissible. Private agency rules need a legal review before they ship
  (Ley 23.592); the team will revisit this.
- A missing qualification fact yields `unknown`, never `eligible`. A discretionary rule, or an income band
  straddling the threshold, yields `conditionally_eligible`, except for a searcher who declared none of
  the fact and clears the rule: the owner's call is not about them (specs/004, decision 2).
- Re-running extraction is deliberate and non-reproducible, like `cmd/listings parse`. Fixing evaluation
  bugs never requires re-extraction.
