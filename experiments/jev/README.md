# Jev matching smoke test

Contract: [MATCHING_CONTRACT.md](../../docs/MATCHING_CONTRACT.md).
This directory contains independently specified cases and a Go smoke runner,
not a completed quality benchmark or a production integration.

From the repository root:

```bash
go run ./experiments/jev -provider=baseline
# Requires AI_GATEWAY_API_KEY in the process environment and a verified account:
go run ./experiments/jev -provider=jev
```

The runner does not automatically load `.env`; use the project's environment
loader or export the variable in the terminal running the command. Credentials
are never CLI arguments or included in output. Jev mode makes external calls;
baseline mode has no network dependency. Reports go to stdout and include the
inventory hash, expected/actual assessments, case success, errors and latency.
Case mismatches are report data (exit 0); setup/report failures exit nonzero.
Every case runs and reports its own error; provider requests carry opaque
candidate IDs, never case names. A `needs_review` assessment never passes a case. Usage/cost telemetry is not implemented.

The baseline completed all four cases. It returns unknown for unsupported prose,
so two cases match all expected labels and two do not. In the real listing it
correctly leaves pets/quietness unknown but cannot recover light/professional
claims absent from parsed attributes. These are expected baseline limitations,
not evidence about Jev accuracy. Tests validate harness behavior, not that every
baseline answer matches the semantic gold label.

Frozen inventory: 300 rows (100 Congreso, 100 Palermo, 100 Monserrat).
SHA-256: `36badfa2d40fab0864eb8c76319ae7de432224b5317c081abfe94cb57c13680a`.
Caballito has no rows in this snapshot: Palermo/Caballito tests must report
Caballito as empty, while Congreso/Monserrat exercises two populated branches.

## Protocol

1. Freeze the SHA-256 of `data/listings.parsed.jsonl` and load rows by exact URL.
2. Read `cases.json`; expected answers never enter the provider request.
3. Send full listing prose as state to Gateway `/v1/evaluate` with
   `typesafe-ai/jev`, one Choice per criterion using the four contract labels.
4. Record answers, distributions, elapsed time, usage and reported cost.
5. Compare each answer against the prewritten expectation. Missing answers
   and transport failures are failures, not correct unknown classifications.
6. Repeat with evidence attribution questions. Check that cited text supports
   the conclusion, not merely that an evidence ID exists.
7. Repeat identical requests and then evaluate a different batch arrangement.
   Keep per-run results; do not hide disagreements behind an average.

The first live request used the real Congreso case's four questions with the
full description. Expected answers were established from its text before
calling Jev: supported light, contradicted professional use, insufficient
evidence for pets and quietness. A disposition or a location is not evidence
that an apartment is quiet.

`search_plan_cases` test intake separately. They are not questions for Jev.
Use inventory neighborhoods with actual rows for nonempty branch coverage;
Palermo/Caballito may intentionally exercise the empty-branch path depending
on the frozen inventory. This smoke set is not a held-out quality benchmark.

## Live attempt — 2026-09-23

- Credentials loaded from workspace `.env`, never printed or persisted here.
- HTTP 403, error type `customer_verification_required`.
- Gateway message: a valid credit card must be on file to service requests.
- Wall-clock request duration: 953 ms; this is rejection latency, not inference.
- No model answer, token usage, or cost returned.
- No claims about model accuracy, speed or savings can be drawn from this run.

Billing verification was resolved by the account owner the same day.

## First answered run — 2026-09-23

One run, four requests, no transport or schema errors. Three of four cases
match their labels. The Congreso light criterion is `needs_review`: the overall
answer was `supported` (p=0.72, conflicting 0.24) while per-record answers
implied `conflicting_evidence` (`exposure=interno` and "da a espacio interior"
alongside "luminoso"). Probabilities arrive rounded to two decimals. Latency:
4.1 s for 40 questions, 0.4–2.6 s for the 2-question synthetic cases. A single
run is a smoke test: no stability, accuracy or cost claim follows from it.

## Implementation sequence

The three test seams were approved and their initial tests pass: matching
behavior, HTTP adapter validation, and branch-preserving shortlist construction.
The experiment runner also has an offline test. Buyer chat integration follows measured results and
must retain both requested branches and the same ranking shown on cards.
No database load, scrape, reparse, or production switch is needed for this
experiment.
