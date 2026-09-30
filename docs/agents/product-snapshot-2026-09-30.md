# Historical product snapshot

Extracted from the previous AGENTS.md on 2026-09-30. Claims and schedules below have not been refreshed. This file supplies historical context only; it is not agent instructions.

## Product

**Hausy** (internal: Project Angus). Rental search that resolves eligibility at discovery.

- **Scope:** CABA rentals first; sales later on the same schema.
- **Problem:** fit is not scarce; eligibility breaks the search. Requirements (guarantee type,
  income multiple, deposit, pets) surface at the transaction, after the visit.
- **Solution layers:**
  1. Multi-portal aggregation — table stakes, not the wedge.
  2. Deterministic eligibility: `eligible` · `ineligible` · `conditionally_eligible` · `unknown`.
  3. Qualified brief to the agency, sent only with consent.
- **Hero path:** zero-results engine. Promise: best *evidenced* fit, not perfect match.
- **Never:** negotiate, charge on the operation, assess people, sell ranking, send without
  per-inquiry consent.
- **Positioning:** Roomix solved discovery; MOB solved qualification at the transaction;
  nobody qualifies at discovery.

Source of record: `idea-validation-agents/memory/ideas/angus-buscador-decision-caba/`
(`idea.md` v8). Update this section from there, not from memory.

### Hypotheses (latest recorded data: 2026-09-09)

| Hypothesis | Status | Evidence |
| --- | --- | --- |
| Guarantee requirements are never published | Falsified | 2026-08-29 |
| Requirement coverage in listings | Verified, n=300 Zonaprop | 33.3% publish requirements; 2.7% income multiple; 5% hard condition |
| Eligibility pain exists (both sides) | Supported as problem, open as business | Survey |
| Guarantee supply vs. information (F-003) | **Open — only hypothesis that kills the product** | Realtor interview planned 2026-09-15; no result recorded |
| Agencies will maintain their rules | Untested bet | Gate 4 |
| Searchers complete intake before results | Stated intent only | Survey |
| Primary payer · acquisition channel · published rules bind | Open | — |
| Multi-criteria preference matching as wedge | Open, parked | — |

### Decisions

- **Hybrid A2A:** typed JSON contracts, deterministic SQL/geo pruning. LLMs only for intake,
  requirement parsing, exceptions, explanations. Free-text agent-to-agent chat: discarded.
- **Agent topology — contested (F-007):** per-agency seller agent vs. shared policy layer. Recent
  reviews propose a shared service with no per-agency agent in v1. Not decided.
- **Seller agent:** internal component for `conditionally_eligible` only; not demoed.
- **December demo:** searcher side only, real scraped public listings, no agency on stage.
- **Scraping public listings:** accepted and sufficient for v1 coverage.
- **Dead:** mail forwarding, building on Tokko API, pure redirect to listing, caución origination
  as main business, agency rejection-handoff channel.

### MVP (December)

- Ingestion + normalization across 3 portals (live scrape + frozen snapshot, same interface).
- Deterministic hard constraints + geo.
- Ranked results with explanations.
- Zero-results engine.
- Brief generation.
- Qualification module is the only swappable part.

| Milestone | Date |
| --- | --- |
| Gate 4 — agency rule loading + shadow test | 2026-09-25 |
| Gates 1–2 | 2026-09-30 |
| Schema freeze | ~2026-11-01 |
| Code freeze | 2026-12-01 |
| Deliverable | 2026-12-15 |
| Gate 3 — intake completion ≥ 40% | Post-launch |

Constraints: 4-person team, USD 50/month budget.

### Market

- **Roomix:** 500k+ listings, 380k visits/month, USD 500k raised, agency WhatsApp agent, Tokko
  integration. Claims guarantee filtering; manual tests found no explicit control and dropped
  constraints.
- **MOB:** qualification at transaction. **Tokko:** leading CRM (QuintoAndar). **Zonaprop:**
  QuintoAndar; named the durable threat.
- CABA rental supply +11.1% YoY · ~28% advertised-vs-signed price gap · caución 4–7% of contract ·
  Ley 5859 bans information fees charged to tenants.

### Survey (n=50, 2026-08-28 → 2026-09-09)

Other files cite 46/48 responses (earlier export cuts).

**Searchers (n=21)**
- Discarded a property over guarantee type: 13 (11 several times).
- Advanced, then found they didn't qualify: 9.
- Top criteria: price/expensas 20 · transit 16 · security 13 · size 11 · light/amenities 11 ·
  commute 8 · pets 8.
- Uses 2+ portals: 18. Saw duplicate listings: 14.
- Would give at start: guarantee type 16 · income band 13 · caución quote 12.
- Would share a summary with the agency: 18 (12 "depends on the property"). Main condition:
  optional fields (13). Main fear: data reused (14).

**Agencies (n=6)**
- 4 get enough inquiries, most unqualified; all 6 lose 30 min–2 h+/day on them.
- 4 spend < USD 100/month. Would pay for unverified lead: 3 yes · 2 no · 1 maybe.

**Homeowners (n=19):** 11 unsure of their property's market value.

