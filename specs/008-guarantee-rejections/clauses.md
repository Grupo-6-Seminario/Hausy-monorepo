# Spec 008: matched guarantee-rejection clauses

This is the hand-review record for acceptance item 4. The matcher is `eligibility.PublishedRules` → `guaranteeRejectionRule`. It ran over the 300 committed listings on 2026-10-23, and Martin reviewed the list before the matcher was written.

## With the rule

| id | Listing | Searchable | Rule | Evidence |
| --- | --- | --- | --- | --- |
| 235 | 59787969 | yes | `guarantee none_of [caucion]`, hard | No se aceptan seguros de caución |

## Near misses (no rule)

A wider scan looked for "no", "sin", "excepto", "salvo" or "ni" within a few words of a guarantee word. It found two more listings. Neither states a rejection, so neither gets the rule.

| id | Listing | Clause | Why |
| --- | --- | --- | --- |
| 237 | 55770968 | No requerimos garantía inmobiliaria. | The listing waives the guarantee; it does not refuse an instrument. |
| 252 | 55360032 | No requerimos garantía inmobiliaria. | Same as 237. |
