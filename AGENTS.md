# Startup Context

> **This file is the single source of truth for project context.** `CLAUDE.md` and `GEMINI.md`
> import it, `.cursorrules` and `.github/copilot-instructions.md` point at it, and Codex reads
> it directly. Edit `AGENTS.md`, never the pointer files. The repo contains no symlinks, so it
> clones correctly on Windows.

> **Setting up a fresh clone:** [docs/SETUP.md](./docs/SETUP.md). It is written to be handed
> straight to an agent.

> **Repo state:** `cmd/hausy` serves the buyer agent over a local HTTP API, backed by the
> OpenAI-compatible model client in `internal/local`. The frontend proxies prompt messages to
> that API. `cmd/listings` parses and loads scraped ZonaProp inventory into Postgres
> (`docker compose up -d`); the schema and attribute vocabulary are documented in
> [docs/DATA_MODEL.md](./docs/DATA_MODEL.md). The AgentCore harness remains available in
> `internal/agentcore`; there is no IaC or CI yet.

> **Reading the inventory:** `internal/search` is the read side — the query a buyer agent can
> ask, and the four tools (`list_neighborhoods`, `search_listings`, `get_listing`,
> `neighborhood_price_stats`) that expose it to a model. `internal/tools` is the
> provider-neutral registry and call loop underneath, so the same toolset backs the local
> OpenAI-compatible endpoint today and a Bedrock agent later. Hard constraints are SQL,
> preferences only reorder — see [docs/DATA_MODEL.md](./docs/DATA_MODEL.md).

## The listing pipeline

Seller-side inventory reaches the database in three steps, deliberately kept separate:

```
scrape  →  data/listings.jsonl         raw page text, no interpretation
parse   →  data/listings.parsed.jsonl  local model reads the prose  ← committed
load    →  Postgres                    deterministic, idempotent upsert on url
```

```bash
docker compose up -d
cd experiments/web-scraper && go run .            # ~300 listings, needs a human for Cloudflare
go run ./cmd/listings parse                       # slow, resumable, non-deterministic
go run ./cmd/listings load                        # fast, repeatable
```

**Both JSONL files are committed.** A model run is not reproducible across machines, so if
every teammate parsed locally they would each get a different reading of every listing.
Committing the parsed file means `load` alone rebuilds an identical database — no scrape, no
model, no Cloudflare challenge. Re-scraping and re-parsing are occasional, deliberate acts.

The split between the three steps is also what makes a normalization bug cheap: fixing
`listing.Normalize` and re-running `load` costs nothing, where baking the parsing into the
scraper would mean visiting 300 pages again.

**What is deterministic and what is not.** Price, expensas, m², ambientes, dormitorios, baños,
cocheras, antigüedad and the Frente/Contrafrente disposition are read off the page as text and
converted by pure functions in `internal/listing`. The model is used only for the qualities
that exist nowhere but the prose — light, noise, condition, amenities, transit. A published
field always outranks the model's reading of the same quality.

`internal/` is production code and carries the conventions below. `experiments/` holds
throwaway spikes — `web-scraper/` is its own Go module and is not part of the main build.
Account-specific values (harness ARN, database URI) live in `.env`; see `.env.example`.

We are exploring a startup idea around improving the apartment rental and purchase search experience in Argentina.

The core problem is that people looking to rent or buy property struggle to reliably identify which available options best fit their actual needs. Real-estate information is fragmented across multiple portals, real-estate agencies, private listings, and other sources. Existing platforms mostly rely on structured filters such as price, location, number of rooms, and amenities, but real decisions also depend on personal and contextual factors such as commuting time, frequently visited places, transportation preferences, neighborhood characteristics, noise, natural light, expenses, flexibility on certain requirements, and other individual priorities.

As a result, users currently have to manually search, compare, normalize, and relate information from many different sources. This makes the process time-consuming, fragmented, and difficult to evaluate confidently.

An early version of the idea was similar to a "Trivago for apartments": aggregate listings from different real-estate portals, detect duplicate properties, compare prices across sources, and help users understand whether a property is actually a good opportunity.

However, existing companies such as Roomix already cover part of this aggregation problem, so we are exploring a more AI-native architecture.

The current hypothesis is an agent-to-agent, or A2A, real-estate marketplace.

Instead of maintaining one centralized database containing every listing, each real-estate agency, developer, broker, or seller could be represented by an autonomous seller agent. That agent would know or have access to the inventory of the organization it represents.

The user would also have a buyer agent representing their preferences, constraints, priorities, and context.

A simplified interaction would look like this:

User → Buyer Agent → Seller Agents → Seller Inventory

The buyer agent could express nuanced intent such as:

"I want a two-bedroom apartment in Palermo or Belgrano, preferably under USD 1,000. Natural light and low noise matter more to me than having a balcony. I work from home, want to be reasonably close to Subte D, and I am willing to stretch my budget slightly for an exceptional property."

Seller agents would interpret the request, search their own inventory, and respond with exact or partial matches.

The buyer agent could then ask follow-up questions, compare alternatives, negotiate trade-offs, normalize information, detect duplicates, and rank the options according to the user's real preferences.

The important idea is that this should not just reproduce a traditional API query using AI.

If every interaction can be reduced to:

bedrooms = 2
location = Palermo
max_price = 1000

then an ordinary API would be better.

A2A becomes interesting when agents can reason about fuzzy preferences and trade-offs.

For example:

Buyer Agent:
"Natural light matters more to me than having a balcony."

Seller Agent:
"I do not have an exact match, but I have a seventh-floor northwest-facing apartment with floor-to-ceiling windows. It has no balcony but may satisfy the reason you wanted one."

Buyer Agent:
"How noisy is it, and what are the expenses?"

Seller Agent:
"It is interior-facing, broker notes describe it as quiet, and current expenses are ARS X."

This creates a discovery experience closer to how human real-estate agents interact with buyers.

The broader thesis is:

Instead of humans manually browsing inventory, buyer intent could be represented by autonomous agents while seller-side agents compete to satisfy that intent.

This creates a reverse-discovery or RFQ-like marketplace:

Buyer intent
→ multiple seller agents respond
→ buyer agent evaluates, verifies, compares, and filters
→ user receives a small set of relevant candidates

Potential benefits include reducing the need for centralized scraping, allowing sellers to keep their inventory private, enabling richer preference matching, and creating a more dynamic interaction between demand and supply.

However, this architecture has important risks that should remain central during development.

The biggest risk is participation and cold start. A2A does not eliminate the data-access problem if the product requires many real-estate agencies to independently deploy agents before the marketplace becomes useful.

A more plausible bootstrap model is that our platform could create and host seller agents for agencies. An agency could connect its CRM, database, feed, or manually provided inventory, and the platform would expose an agent on its behalf.

Their incentive would not be "join an A2A network." Their incentive would be receiving better-qualified leads and automating repetitive property enquiries.

Another major problem is trust. Seller agents are economically incentivized to present listings positively, so the buyer agent should distinguish between seller claims and verifiable facts.

Examples:

Seller claim:
"Very bright."

Verifiable information:
orientation, floor, window size, surrounding buildings, photos, etc.

Seller claim:
"Low expenses."

Verifiable information:
current expenses compared with similar buildings or neighborhood averages.

A future platform could therefore provide more than communication. It could become a trust and coordination layer providing:

- agent discovery
- identity
- structured property schemas
- duplicate detection
- listing normalization
- reputation
- verification
- historical pricing
- market comparison
- routing between buyer and seller agents

Potentially, A2A would be used between independent organizations, while MCP, APIs, or tools could be used internally by each seller agent to access CRMs, listing databases, calendars, broker notes, or other systems.

For the initial MVP, we explicitly do NOT want to solve the entire real-estate ecosystem.

The MVP should contain:

- one buyer agent
- one seller agent
- a small local or synthetic property inventory, for example JSON or SQLite
- an A2A interaction between them
- natural-language buyer requirements
- seller-side matching
- follow-up questions and negotiation around preferences
- buyer-side ranking and reasoning

There is no need initially for:

- scraping
- real real-estate portal integrations
- many seller agents
- payments
- production-grade authentication
- a complex frontend
- a global agent registry
- reputation systems
- real broker integrations

The goal of the MVP is to test one core hypothesis:

Can autonomous buyer and seller agents communicate nuanced user intent and property trade-offs in a way that creates a meaningfully better property-discovery experience than traditional database filters?

When helping with this project, do not assume that A2A is necessarily the correct solution. This project is still in idea validation and may currently be a Solution In Search of a Problem.

Challenge architectural choices when appropriate.

Separate the underlying user problem from the proposed technical solution.

Prefer simple deterministic software when AI is unnecessary.

For example, use normal database filtering for explicit constraints such as price, bedrooms, and location, and use LLM reasoning for ambiguous preferences, trade-offs, natural-language interpretation, follow-up questions, and ranking.

Avoid using an LLM where a normal query or function would be more reliable.

Also keep the following startup validation principles in mind:

- Do not optimize for the novelty of A2A itself.
- Ask whether AI enables something that traditional software could not reasonably provide.
- Identify assumptions that must be true for the business to work.
- Prefer experiments that can cheaply falsify those assumptions.
- Avoid building expensive infrastructure before validating user demand.
- Competition such as Roomix does not invalidate the problem, but it means aggregation alone is probably not enough differentiation.
- The strongest current opportunity may be decision support rather than raw listing aggregation.
- A strong product should help users determine not merely "what properties exist" but "which property is actually right for me and why."

Possible future directions include:

- fair-value estimation
- identifying unusually cheap or expensive listings
- cross-portal price comparison
- duplicate property detection
- historical price tracking
- neighborhood intelligence
- commute analysis
- natural-language property search
- autonomous communication with brokers
- visit scheduling
- user preference learning
- seller-side lead qualification
- agent reputation and trust
- negotiation between buyer and seller agents

Treat these as hypotheses, not requirements.

The current working project objective is:

Build and evaluate a small A2A prototype in which a buyer agent represents nuanced real-estate preferences and communicates with a seller agent that controls a property inventory, allowing both agents to exchange information, reason about trade-offs, and identify the properties that best fit the user's needs.

## Engineering conventions

### Test-driven development

Every change follows the **red → green** loop: write the failing test first, then only enough
code to pass it. Tests run with `go test ./...` — Go's stdlib `testing`, no framework to add.

Work in **vertical slices**: one test → one implementation → repeat. Each test is a **tracer
bullet** that responds to what the last cycle taught you, rather than one of a batch written up
front against imagined behaviour.

Before writing a test, name the **seam** under test — the public boundary where behaviour is
observable without reaching inside — and confirm it. Tests live at seams, so agreeing them up
front is what puts the effort on matching, ranking, and negotiation logic instead of on every
edge case.

Expected values come from an independent source of truth: a known-good literal, a worked
example, the spec. A test that recomputes the expected value the way the code does passes by
construction and can never disagree with it.

Refactoring belongs to review, not to the loop.

For the full reference — what a good test is, mocking, anti-patterns — invoke the
`mattpocock-skills:tdd` skill.
