---
name: eligibility-verdicts
description: "Check a change to the eligibility evaluator (Assess, outcome, the operator comparisons, Relaxations in internal/eligibility/eligibility.go) against eight promises over every input in a bound. Use when a diff touches that code, adds an operator, fact or guarantee instrument, or changes the eligibility_facts migrations."
---

# Eligibility verdicts

`eligibility.Assess` decides whether a searcher sees a listing as "Calificás", "Calificás con condiciones", "A confirmar", or not at all ("No califica"). `eligibility.Relaxations` writes the "Si conseguís X, vuelven N" line. `internal/eligibility/verdicts_formal_test.go` runs both on every input inside a bound and checks eight promises. A wrong verdict either hides a listing the searcher could rent or tells them they qualify when nothing says so.

## Does your change touch it?

Run the check if your diff touches any of these:

| Covered code | What the check relies on |
| --- | --- |
| `internal/eligibility/eligibility.go`: `Assess`, `outcome`, `comparison`, `oneOf`, `noneOf`, `subsetOf`, `incomeMultiple`, `incomeThreshold`, `hasReason`, `Relaxations` | The code under check. |
| `internal/eligibility/eligibility.go`: `ValidateQualification`, the `Operator`, `Hardness` and `Reason` constants | Builds the bounded qualifications and rules. |
| `internal/eligibility/extract.go` (`Extract`, `Instruments`), `PublishedRules`, `incomeProofRule`, `guaranteeRejectionRule`, `petsRule` | Which rule shapes exist. The check generates those shapes; it does not run these functions. |
| `internal/store/postgres/migrations/*eligibility*`, `0008_remove_caucion_quoted.sql` | `formalCatalog` copies the facts and choices they create. |

**Last confirmed at commit `a37b2b4`.** To see what changed since then, run `git log --oneline a37b2b4..HEAD -- internal/eligibility internal/store/postgres/migrations`. Move this commit forward only after you have reread the changed code and confirmed that the bound, the catalog and the eight promises still describe it.

## Run it

```sh
go test -tags formal -run TestFormal -count=1 ./internal/eligibility
```

It takes about 2 seconds and checks about 977,000 inputs. The `formal` build tag keeps the check out of `go test ./...` and CI while finding K1 below is open. On a pass, `-v` prints the input count.

Each failing promise prints one counterexample: the searcher's qualification, the listing, the rules, the verdict, and what the promise expected.

## The eight promises

1. A rule on an inadmissible fact (age, nationality) never changes the verdict. Source: ADR 0001.
2. A fact the searcher never declared never gives "Calificás" or "Calificás con condiciones". A listing with no admissible rules is "A confirmar". Source: ADR 0001.
3. "No califica" happens exactly when the searcher fails a hard rule. A discretionary rule never hides a listing. Source: `CONTEXT.md`, "Conditionally eligible".
4. "Calificás" needs every rule cleared, and at least one of them cleared by a rule that accepts something. A refusal (`none_of`) alone never confirms a guarantee. A refusal that doesn't name all of the searcher's guarantees is settled by any other cleared rule, whatever its fact. Source: specs/008 and the user's decision of 2026-10-26.
5. The order of the rules never changes the verdict.
6. Declaring one more guarantee never hides a listing that was shown.
7. The relaxation line names exactly the instruments that a rule accepts and whose declaration shows the hidden listing. Declaring an instrument replaces "No tengo". Source: specs/008.
8. A searcher who clears every rule, all of them hard, sees "Calificás". Each rule must accept them outright, or be a refusal that doesn't refuse everything they declared, and at least one rule must accept. Without this promise, an evaluator that never answers "Calificás" passes the other seven.

The test's `fails` and `clears` functions are the glossary's meaning of failing and clearing a rule. They're written apart from `outcome` on purpose. Keep it that way.

## Known findings

The current code fails one promise. Expect this failure until the user decides it.

- **K1, promise 7.** `Relaxations` appends the suggested instrument to `["none"]` instead of replacing "No tengo". With `["none", "propietaria"]`, a hard `none_of [propietaria]` rule reads as unconfirmed instead of failed. A listing whose rules both accept and refuse propietaria is then counted as one that "vuelve", but declaring propietaria leaves it hidden. Every K1 counterexample has a searcher with `guarantee: [none]` and a hard refusal of an instrument that another rule accepts.

Any other failure is new. To check whether a promise-7 failure is K1, rerun with property 7's comparison temporarily skipped for `q["guarantee"]` equal to `["none"]`. Revert that edit afterwards.

## Real bug or check error?

A counterexample is a check error when one of these holds:

- The check compares something that isn't part of the verdict a reader sees, such as pointer addresses or ordering inside a slice. `summary` and the sorted `got`/`want` lists exist for this reason.
- The bound generated an input the product can never produce, such as a qualification `ValidateQualification` refuses or a rule shape no loader writes. Read the loaders listed above before you decide.
- `fails` or `clears` disagrees with `CONTEXT.md` or the specs.

Otherwise it's a real bug. Report it in the counterexample's terms. Don't weaken the promise or narrow the bound until it passes. Don't change product code unless the user asks.

## When a change is intended

- **New operator, fact, choice or instrument:** add it to `boundedRules`, `formalCatalog` or `instruments`. Add its case to `fails` and `clears` from the spec. Then break the new code on purpose once and confirm that a promise catches it.
- **Changed product decision:** change the promise only after the user states the new behavior. Record the source next to it, as promise 4 does.
- **Before you finish:** move the confirmed commit forward, as described above.

## Bound and what it leaves out

- **Rules:** no rules, one rule, or any ordered pair, drawn from 78 shapes:
  - guarantee `one_of` and `none_of` over every non-empty set of the three instruments
  - pets `subset_of` over every non-empty set of the four choices
  - income_documented `one_of`
  - `income_multiple` and `income_multiple_with_expenses` with multiples 2, 3 and an unreadable one
  - all of the above at both hardness levels, plus one unknown operator and one rule on `age`

  Three or more rules aren't checked. The verdict is a precedence over per-rule reasons, so pairs exercise every interaction between two reasons.
- **Qualifications:** every valid declaration of the facts the rules name, each undeclared or declared. Other facts stay undeclared, because `outcome` reads the qualification only at `q[r.Fact]`.
- **Listings:**
  - rent ARS 500,000 with ARS 200,000 expensas
  - rent in USD
  - rent with no expensas
  - no rent

  Assess reads nothing else from a listing.
- **Relaxations:** checked one candidate at a time. Counts summed across many listings and the sort order aren't checked.
- **Not covered:**
  - how rules are read from text (`PublishedRules`, `Extract`, `IncomeBase`)
  - how `internal/buyer/pipeline.go` hides, sections and ranks verdicts
  - the store
  - the wording of conditions
