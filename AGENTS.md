# AGENTS.md

> Single source of truth for agent context. `CLAUDE.md` and `GEMINI.md` import it;
> `.cursorrules` and `.github/copilot-instructions.md` point at it. Edit this file only.
> Fresh clone: [docs/SETUP.md](./docs/SETUP.md).

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

## Repo

```
scrape → data/listings.jsonl → parse (local LLM) → data/listings.parsed.jsonl ─┬→ load → Postgres
                                  eligibility (Jev) → data/listings.eligibility.jsonl ─┘      ↓
frontend (/api/agent) → cmd/hausy POST /api/messages → internal/buyer pipeline:
    internal/intake plan → Postgres candidates → internal/eligibility → order → reply (HAUSY_LLM)
frontend (/api/agency/catalog) → cmd/hausy → internal/agency.Catalog → Postgres
```

After the first inventory audit, `cmd/listings audit` reads stored descriptions
from Postgres and writes `data/listings.quality.jsonl`; `load` restores those
review decisions after the other JSONL files. Each decision is bound to a
content fingerprint, so changed listings remain pending.

| Path | Role |
| --- | --- |
| `cmd/hausy` | HTTP API serving the buyer agent |
| `cmd/listings` | `parse`, `eligibility` and `load` subcommands |
| `internal/buyer` | Buyer turn: plan → candidates per branch → eligibility → order → one reply call |
| `internal/intake` | Planner: conversation → typed plan (branches, sort, volunteered qualification); Jev with the `HAUSY_LLM` model as fallback, or that model alone (`HAUSY_PLANNER`) |
| `internal/eligibility` | Pure eligibility evaluator (four states), zero-results relaxations, rule extraction at load time ([ADR 0001](./docs/adr/0001-eligibility-rules-as-data.md)) |
| `internal/jev` | Vercel AI Gateway transport for Jev: retries, sanitized errors |
| `internal/search` | Read side: `search.Query`, validation, store read contract |
| `internal/llm` · `internal/local` · `internal/bedrock` | Vendor-neutral LLM types · OpenAI-compatible client · Bedrock Converse client; `HAUSY_LLM` picks one |
| `internal/listing` | Listing model, deterministic parsers |
| `internal/agency` | Transport-neutral realtor catalog commands and contact-intent contract; HTTP and a future A2A adapter share this seam |
| `internal/auth` | Account seam: `Provider` (sign-up/in/out, bearer tokens), `Local` + memory store; a Cognito `Provider` plugs in at `cmd/hausy` |
| `internal/pipeline` · `internal/store/postgres` | Parse/load steps · persistence (listings, users, sessions) |
| `frontend/` | Next.js via vinext; proxies to `HAUSY_BACKEND_URL` (default `127.0.0.1:8080`). Design: `frontend/DESIGN.md` |
| `specs/` | Feature specs (`001-search-ui`, `002-eligibility-first-search`) |
| `experiments/` | Throwaway spikes; `web-scraper/` is its own module |
| `infra/` | Terraform for the AWS account: one root module, state in S3; apply from `dev` after merge |

- Domain glossary: [CONTEXT.md](./CONTEXT.md). Decisions: [docs/adr/](./docs/adr/).
- Schema and attribute vocabulary: [docs/DATA_MODEL.md](./docs/DATA_MODEL.md).
- Realtor catalog and contact-intent API: [docs/AGENCY_CATALOG.md](./docs/AGENCY_CATALOG.md).
- Account-specific values live in `.env` (see `.env.example`).
- AWS infrastructure is Terraform under `infra/` ([ADR 0002](./docs/adr/0002-infrastructure-as-terraform-in-this-repo.md)). No CI.

### Data rules

- **All four JSONL files are committed.** Model parsing and quality review are not reproducible; `load` alone must rebuild
  an identical database.
- **Deterministic fields** (price, expensas, m², rooms, baths, parking, age, disposition) are
  parsed by pure functions in `internal/listing`.
- **At parse time, the LLM reads only prose-only qualities** (light, noise, condition, amenities, transit). The separate quality audit compares stored prose with published facts.
- A published field always outranks the model's reading.
- Normalization bugs: fix `internal/listing`, re-run `load`. Never re-scrape to fix parsing.

```bash
docker compose up -d
go run ./cmd/listings load     # fast, idempotent
go run ./cmd/listings parse    # slow, resumable, non-deterministic — deliberate only
go run ./cmd/listings eligibility  # Jev; slow, resumable, non-deterministic — deliberate only
go run ./cmd/listings audit        # review stored DB descriptions; writes committed quality snapshot
```

## Engineering workflow

Every task runs these steps in order. The steps are mandatory; skills are personal (see
[Skills](#skills)). When a named skill is installed, use it for that step; otherwise follow the
step as written.

### 1. Comprehend — always

- Use **`codebase-design`** at the start of every task.
- Before writing any code, trace the affected flow end to end: every module, caller, and test it
  touches.
- Read the relevant docs (`docs/DATA_MODEL.md`, `specs/`, `frontend/DESIGN.md`).
- State the modules, interfaces, and seams involved, and where the change belongs.
- No implementation starts until this is written down. A small diff in the wrong module is a
  second bug.

### 2. Bugs — always diagnose first

- Any bug, failure, regression, or "this is wrong" uses **`diagnosing-bugs`**.
- Build a red-capable feedback loop before forming a hypothesis.
- Fix the root cause at the shared seam, not the reported caller.
- State the confirmed hypothesis in the PR.

### 3. Build — TDD

- Use **`tdd`**: red → green, one vertical slice at a time.
- Tests live at the seams agreed in step 1.
- Expected values come from an independent source (literal, worked example, spec). Never
  recompute them the way the code does.
- Refactoring belongs to review, not the loop.
- Go: `go test ./...` (stdlib `testing`). Frontend: `npm test` in `frontend/` (vitest).

### 4. Design rules

- **Hard constraints are SQL; preferences only reorder.** Price, rooms, location, eligibility
  rules: deterministic code.
- LLMs only for ambiguity: intake, prose parsing, exceptions, explanations.
- If a normal query or function works, don't use a model.
- Keep open hypotheses open: don't build per-agency agents or negotiation while F-007 is
  undecided.
- Challenge scope that doesn't serve a hypothesis or the December MVP.

### 5. Ship — `/ship`

- Work on feature branches in Orca worktrees based on `dev`. Never commit to `dev` or `main`.
- `/ship` runs tests, commits, pushes, and opens (or updates) a PR into `dev`. PRs are never
  drafts.
- Every PR body has a `## Diagram` (Mermaid, call tree, file tree, or diff sketch), drawn with
  **`show-me`**.

### Skills

Skills are personal: each teammate installs their own, and none are committed except `ship`
(team workflow). Install at user level (`~/.claude/skills/`) or locally, hidden from git via
`.git/info/exclude`. Orca worktrees only contain committed files, so link local skills in with
the Orca repo setup script. Never commit a skill you are trying out.

Skills this workflow refers to:

| Skill | When |
| --- | --- |
| `codebase-design` | Start of every task |
| `diagnosing-bugs` | Every bug |
| `tdd` | Every code change |
| `show-me` | Every PR diagram; explaining structure |
| `ship` | Finishing a task (`/ship`) |
| `ponytail` | Choosing the smallest implementation, *after* comprehension, in pair with tdd|
| `code-review` | Reviewing a branch or PR |
| `improve-codebase-architecture` · `grill-with-docs` · `to-spec` | On request |
| `research` | Primary-source investigation |
| `amazon-bedrock` | Only for explicit Bedrock/AgentCore work |
