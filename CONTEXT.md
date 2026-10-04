# Hausy

Rental search that resolves, at discovery, whether a property fits what the searcher needs and whether the searcher can actually rent it.

## Language

### Searching

**Requirement**:
A quality the searcher treats as necessary, or rejects, in the property they are looking for, in their own terms: "quiero que sea luminoso", "con cochera", "olvidate de la cochera" (no cochera). A listing without support for a required quality is not an exact match.
_Avoid_: criterion, wish, need

**Dormitorio**:
A bedroom. "Habitación" means the same, so "dos habitaciones" is two dormitorios, never two ambientes. An **ambiente** counts every room, living room included; a monoambiente is one.
_Avoid_: habitación as a room count of unclear kind

**Unresolved search condition**:
A stated condition with multiple plausible meanings that would materially change the search, where Hausy cannot safely choose for the searcher ("con amenities" without saying which, "puedo pagar hasta 900" without a currency).
_Avoid_: subjective quality, invalid filter, missing listing data

**Unsupported condition**:
A stated hard condition Hausy has no way to evaluate on any listing, such as "cerca del trabajo" while search knows no work location or distance. The search stops with an explanation, and the searcher edits or removes it.
_Avoid_: unresolved search condition, missing listing data

**Unconfirmed requirement**:
A requirement that a particular listing neither supports nor contradicts, because the ad says nothing about it. The listing is still shown, marked as unconfirmed for that requirement, and ranked after listings that confirm it. It never stops the search.
_Avoid_: unsupported condition, mismatch, no data

**Clarification**:
An explicit answer from the searcher that resolves an unresolved search condition or an unresolved qualification. The answer may be a choice or a short value, depending on what the condition needs.
_Avoid_: inferred default, model guess

**Preference**:
A quality the searcher would like but would trade away, as in "idealmente luminoso". It affects ordering without excluding properties.
_Avoid_: soft requirement

**Criterion assessment**:
How the available property evidence supports or contradicts a requirement or preference, or leaves it unconfirmed. A listing claim and an inferred hint remain distinguishable; neither establishes rental eligibility.
_Avoid_: verified physical quality, model confidence as fact

**Preference score**:
An internal comparison of evidence-supported and contradicted preferences, weighted by priorities the searcher explicitly declared. Missing evidence remains part of the requested preference set; the score is neither a public match percentage nor a probability of being able to rent.
_Avoid_: eligibility score, probability of approval

**Qualitative evidence**:
A passage or published field about the particular property used to assess a subjective requirement. A direct claim can support or contradict it in equivalent wording, but only at the scope described ("dormitorio luminoso" is not a claim about the whole home). Structural clues remain hints, distinct from direct support; their ranking contribution depends on the matching policy.
_Avoid_: verified quality, match probability

**Inconsistent listing**:
An ad containing two explicit, incompatible claims about the same property detail at the same time, with both claims traceable to its text. It is withheld from buyer search until the inconsistency is resolved.
_Avoid_: uncertain listing, weakly inferred conflict

**Branch**:
One or more neighborhoods that share the same requirements within a single search; a search has several branches only when the searcher ties different requirements to different neighborhoods.
_Avoid_: sub-search, zone

### Eligibility

**Qualification**:
What the searcher declares about themselves that a property may demand: guarantee instruments, income band, whether they already have a caución quote, their pets. Open-ended; the set of facts grows with the rules that use them. Declaring that one has none ("no tengo garantía", "no tengo mascotas") is a qualification too, and differs from saying nothing: a property that demands a guarantee makes the first searcher ineligible and the second unknown.
_Avoid_: profile, solvency, requirements (of the searcher)

**Unresolved qualification**:
A qualification fact the searcher volunteered without enough detail to evaluate it against a property's eligibility requirement, as in "tengo garantía" without saying which type. Clarifying it is optional for the searcher.
_Avoid_: ineligible, unknown eligibility

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
