---
status: accepted
---

# Propose typed clarifications before search

> The generative proposal below is replaced by [ADR 0004](./0004-jev-judges-ambiguity-schema-supplies-options.md): Jev judges ambiguity and the plan schema supplies the options. The rest of this decision stands.

A valid search plan can still misread or omit a condition the searcher stated. On every new or refined search turn, a generative model examines the conversation, available context, and tentative plan for unresolved search conditions or volunteered qualification, including when Jev made the plan. It proposes a typed clarification with choices or a short value; application code validates that an answer can affect listing membership, order, or eligibility before showing one question at a time and resuming from the user-confirmed answer. A short answer is converted to a typed value and validated; the planner does not reinterpret an option label as a new request.

This adds a model call to Jev turns and can delay results, but catches materially different interpretations that plan validation and zero-result checks miss. If proposal fails, deterministic checks remain; a detected unresolved condition is never silently discarded.

Confirmed answers remain in the current chat and are cleared for a new chat or session. Clarifying qualification is optional: "Prefiero no decir" is an explicit refusal that suppresses repeated questions about that fact for the rest of the chat, not an answer to infer from. A proposed question whose answer cannot change a supported result is rejected before display.

The proposer favors precision: it asks as few questions as possible and only for material ambiguity. For finite, exhaustive choices, a bounded check against the current candidates avoids asking when the alternatives cannot change displayed listings, order, or eligibility. Multiple questions are shown one at a time; hard filters take priority, but their exact order is not a product invariant. While a question is active, ordinary chat submissions pause. "Editar búsqueda" cancels pending questions and returns to the request; "Ninguna de estas" accepts a short, validated answer. An unsupported hard condition stops the search with an explanation and an edit or remove path, while optional qualification may remain unknown.

There is one clarification proposal pass before search, not another model pass after zero results; the zero-results flow handles an empty result. Each question names the user's phrase or known fact that prompted it, so the searcher can recognize a misreading.
