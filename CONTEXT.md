# Hausy

Rental search that resolves, at discovery, whether a property fits what the searcher needs and whether the searcher can actually rent it.

## Language

### Searching

**Requirement**:
A quality the searcher treats as necessary, or rejects, in the property they are looking for, in their own terms: "quiero que sea luminoso", "con cochera", "olvidate de la cochera" (no cochera). A listing without support for a required quality is not an exact match.
_Avoid_: criterion, wish, need

**Preference**:
A quality the searcher would like but would trade away, as in "idealmente luminoso". It affects ordering without excluding properties.
_Avoid_: soft requirement

**Qualitative evidence**:
A passage or published field about the particular property used to assess a subjective requirement. A direct claim can support or contradict it in equivalent wording, but only at the scope described ("dormitorio luminoso" is not a claim about the whole home); structural clues can adjust ranking without establishing a match.
_Avoid_: verified quality, match probability

**Inconsistent listing**:
An ad containing two explicit, incompatible claims about the same property detail at the same time, with both claims traceable to its text. It is withheld from buyer search until the inconsistency is resolved.
_Avoid_: uncertain listing, weakly inferred conflict

**Branch**:
One or more neighborhoods that share the same requirements within a single search; a search has several branches only when the searcher ties different requirements to different neighborhoods.
_Avoid_: sub-search, zone

### Eligibility

**Qualification**:
What the searcher declares about themselves that a property may demand: guarantee instruments, income band, whether they already have a caución quote. Open-ended; the set of facts grows with the rules that use them.
_Avoid_: profile, solvency, requirements (of the searcher)

**Eligibility requirement**:
A rule a property or its agency applies to the searcher's qualification ("acepta garantía propietaria o caución", "ingresos 3x el alquiler"). Either published in the listing or, in a future release, stated privately by the agency and never shown to the searcher.
_Avoid_: listing requirement, condition

**Eligibility**:
Whether this searcher can rent this property, as one of four states: eligible, conditionally eligible, unknown, ineligible. Never a yes/no.
_Avoid_: qualifies, match

**Conditionally eligible**:
The property's condition is discretionary, owner-dependent, or the searcher is near the line; shown with the condition named.

**Unknown (eligibility)**:
Nothing says whether the searcher clears what the property demands: nothing is published, declared or observed about it, or it demands something the searcher has not declared or that cannot be checked (an income multiple on a rent in dollars). Shown and marked, never treated as eligible.
