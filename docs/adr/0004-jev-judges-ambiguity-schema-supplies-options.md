---
status: accepted
---

# Jev judges search ambiguity; the plan schema supplies the options

A clarification question is no longer written by a generative model. On each new or refined search turn, Jev answers one typed yes/no question per plan field that can be read two ways: room kind, generic amenities, budget currency, guarantee type, income. Each question describes the ambiguity by what it means for the search, not by trigger words. For a field Jev flags, code builds the question from the schema's alternatives. It then drops questions that do not apply to the plan (a guarantee the plan already carries, a currency with no budget) and those whose answers do not change the displayed results.

Measured on 21 labeled searches (`experiments/ambiguity`), Jev as judge caught 8 of 10 real ambiguities, including phrasings no question named ("2 cuartos", "todos los chiches del edificio", "hasta 900"), in about 0.4 s. The generative proposers produced typed options the schema does not admit: the local Qwen caught 0 of 4, and Claude Sonnet 4.6 caught 1 of 10 in 2.7 s. The model sees the ambiguity; writing valid options was the failure. The cost is an AI Gateway call per search turn, and an ambiguity about a field the table does not list is not asked, but such an ambiguity cannot change results anyway. This replaces the generative proposal of [ADR 0003](./0003-typed-clarification-before-search.md); the rest of ADR 0003 stands.
