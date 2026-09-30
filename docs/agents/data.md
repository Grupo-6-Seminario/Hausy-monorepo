# Listings, search, and model output

Before changing data or search behavior, read [the glossary](../../CONTEXT.md), [data model](../DATA_MODEL.md), [matching contract](../MATCHING_CONTRACT.md), and relevant ADRs. Preserve their distinctions between requirements, preferences, clarification, qualitative evidence, and four-state eligibility.

## Decisions and evidence

Keep explicit numeric, location, and other evaluable hard constraints in deterministic query/code paths. Preferences affect ordering. Use models for ambiguity, prose interpretation, exceptions, and explanations through replaceable provider boundaries.

A stated condition stays present until clarified or explicitly removed by the searcher. Unresolved meaning, unsupported capabilities, unconfirmed listing evidence, and unknown eligibility are different states. Preserve the existing contract for each rather than guessing, relaxing a condition, or treating missing evidence as confirmation.

Published fields outrank model interpretations. Keep the source passage or field for qualitative assessments, and distinguish direct support from inferred clues. Eligibility has four states; unknown is never silently eligible. Ask before changing ranking, eligibility, hard-condition semantics, or evidence precedence.

## Rebuilds and snapshots

The committed listing, parsed, eligibility, and quality JSONL snapshots are the reconstruction inputs. Preserve their provenance and content-bound quality fingerprints. A normalization bug belongs in deterministic parsing/loading; it does not justify a fresh scrape.

Use existing fixtures and deterministic local checks within approved scope. Ask before re-scraping, regenerating model-derived snapshots, paid model evaluations, or changing committed data. Confirm the destination before any command that mutates an existing database. Use designated disposable test databases for destructive integration checks.

A script finishing successfully proves execution, not listing coverage, correctness, or market representativeness. Record the sample and limits of any audit or evaluation. Separate observed data, interpretation, assumptions, and unknowns.
