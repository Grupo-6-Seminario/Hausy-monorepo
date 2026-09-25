# Listing data model

The contract between the ingestion pipeline and anything that reads listings —
the seller-agent tool layer, a local Qwen tool implementation, a future Bedrock
agent. Schema lives in `internal/store/postgres/migrations/`; the Go types are
in `internal/listing`.

## The one rule that governs everything here

**Columns are facts the page published. Rows in `listing_attributes` are
readings of prose.** A tool must not blur the two. `price_amount` was printed on
the listing; `natural_light = high` is a model's reading of the words "muy
luminoso". Filter hard constraints on columns and treat attributes as evidence
to weigh, never as ground truth.

## Tables

### `agencies` — the inmobiliaria

| Column | Type | Null | Notes |
| --- | --- | --- | --- |
| `id` | bigint identity | no | |
| `name` | text | no | unique; verbatim as published, e.g. `LOUNGE PROPIEDADES S.A.` |

Names are not normalized. The same firm may appear under spelling variants;
deduplication is deliberately not attempted yet.

### `listings` — one property

| Column | Type | Null | Notes |
| --- | --- | --- | --- |
| `id` | bigint identity | no | |
| `source` | text | no | `zonaprop` |
| `url` | text | no | **unique — the identity key.** Ingest upserts on it |
| `neighborhood` | text | no | lowercase slug: `congreso`, `palermo`, `monserrat` |
| `agency_id` | bigint | **yes** | FK → `agencies.id` |
| `address` | text | yes | as published, e.g. `Callao al 200, Congreso, Capital Federal` — block-level, not exact |
| `description` | text | no | seller's prose, paragraph breaks kept |
| `operation` | text | yes | `alquiler` \| `alquiler_temporal` \| `venta` |
| `price_amount` | numeric(14,2) | **yes** | |
| `price_currency` | text | yes | `ARS` \| `USD` |
| `expenses_amount` | numeric(14,2) | **yes** | expensas, almost always ARS |
| `expenses_currency` | text | yes | |
| `total_area_m2` | numeric(10,2) | yes | |
| `covered_area_m2` | numeric(10,2) | yes | |
| `rooms` | integer | yes | *ambientes* — total rooms, Argentine convention: a monoambiente is 1 |
| `bedrooms` | integer | yes | *dormitorios* |
| `bathrooms` | integer | yes | |
| `parking_spaces` | integer | yes | *cocheras* |
| `age_years` | integer | yes | `0` = *a estrenar*; NULL if under construction |
| `floor` | text | yes | text, not a number: `PB`, `7`, `10 A` |
| `scraped_at` | timestamptz | no | |
| `parsed_at` | timestamptz | yes | NULL = attributes were never extracted for this row |
| `parser_model` | text | yes | which model produced the attributes |
| `ingested_at` | timestamptz | no | |
| `owner_user_id` | bigint | yes | FK → `users.id`; set only for account-managed catalog entries |
| `catalog_status` | text | no | `active` \| `archived`; archived rows never appear in buyer search |
| `quality_status` | text | no | `legacy` \| `pending` \| `passed` \| `withheld`; buyer search includes only `legacy` and `passed` |
| `quality_evidence` | jsonb | yes | Two source-backed claims for a withheld listing |
| `quality_checked_at` | timestamptz | yes | Last completed review |

Indexes: `neighborhood`, `(price_currency, price_amount)`, `bedrooms`,
`(owner_user_id, catalog_status)`, unique `url`.

`owner_user_id` is authorization state, while `agency_id` is listing content.
They must not be treated as interchangeable. An authenticated realtor account
owns mutations today; a future organization or A2A identity adapter can map its
principal to the same catalog owner without changing the listing contract.

Removing an entry from a realtor catalog sets `catalog_status = 'archived'`.
It does not delete the canonical facts or contact history.

`quality_status` is a separate publication gate. New listings and changes to
published facts or prose become `pending` and disappear from buyer search until
reviewed. Repeating an unchanged `load` preserves the prior review. Migration
marks existing rows `legacy` so the site stays available while the one-time
audit runs. `go run ./cmd/listings audit` reviews descriptions already in
Postgres with Jev and checks source quotes for suspected contradictions; it
does not scrape or parse the JSONL files. Completed decisions for committed
listings are written to `data/listings.quality.jsonl` and restored by `load`.
Each decision carries a content fingerprint; a changed listing cannot inherit
an old approval and stays pending.
After attempting every legacy row,
the command moves unresolved rows to `pending`. Re-running it resumes those
rows. `withheld` means two explicit claims about the same detail, time, and
scope were verified as incompatible. Geographic district names and simultaneous
sale/rental claims need additional review before they can establish a conflict.

### `listing_contact_intents` — immutable contact-button activations

| Column | Type | Null | Notes |
| --- | --- | --- | --- |
| `intent_id` | uuid | no | PK; caller-generated idempotency key |
| `listing_id` | bigint | no | FK → `listings.id`, `ON DELETE CASCADE` |
| `source` | text | no | currently `search_result_card` |
| `created_at` | timestamptz | no | server timestamp |

The realtor-facing `contact_count` is derived with `count(intent_id)`; no
client can submit or overwrite the aggregate. A retried `intent_id` for the
same listing is a successful replay, while reusing it for another listing is a
conflict. This measures deliberate Contactar activations, not unique people or
delivered inquiries.

### `listing_attributes` — parsed qualities

| Column | Type | Null | Notes |
| --- | --- | --- | --- |
| `listing_id` | bigint | no | FK, `ON DELETE CASCADE` |
| `type` | text | no | from the vocabulary below |
| `value` | text | no | from the vocabulary below |
| `provenance` | text | no | `stated` \| `inferred` — CHECK-enforced |
| `evidence` | text | yes | the verbatim phrase the value came from |

PK `(listing_id, type, value)`; index on `(type, value)`.

A listing carries **multiple rows of the same `type`** where that makes sense
(`amenity`, `suitable_for`, `transit_access`). Single-valued types
(`natural_light`, `exposure`) carry at most one row.

An `amenity` row is stored only when the listing states it in words that name
it: `load` drops inferred amenities and stated ones whose evidence names
something else (`listing.StatedAmenities`), while the parsed file keeps the
model's full reading. A required amenity does not exclude listings: those that
lack the row are shown after the ones that have it, as unconfirmed
([CONTEXT.md](../CONTEXT.md), Unconfirmed requirement).

### Eligibility — `eligibility_facts`, `listing_eligibility_rules`, `user_qualifications`

Why and how: [ADR 0001](./adr/0001-eligibility-rules-as-data.md); terms: [CONTEXT.md](../CONTEXT.md).

| Table | Holds |
| --- | --- |
| `eligibility_facts` | Fact names (`guarantee`, `income_band`, `caucion_quoted`, …), whether each is `admissible`, and the choices the qualification form offers. Protected characteristics (`age`, `nationality`, `gender`) are rows with `admissible = false`. |
| `listing_eligibility_rules` | A listing's eligibility requirements: `fact`, `operator` (`one_of`, `income_multiple`), `values` (JSON), `hardness` (`hard` / `discretionary`), `visibility` (`public` / `private`), `source` (`parsed` / `declared` / `observed`), and `evidence` in the listing's own words. |
| `user_qualifications` | What a signed-in searcher declared: one row per `(user, fact, value)`. |

Rules come from `data/listings.eligibility.jsonl` (`cmd/listings eligibility`, then `load`). No
rules means eligibility is `unknown`, never `eligible`. Instruments are data: adding one means a row
in `eligibility_facts` plus a pattern in `eligibility.Instruments`, and no evaluator change.

## The vocabulary is closed

Anything a model emits outside this set is discarded before it reaches the
database, so a tool can rely on these being the only values that exist. Source
of truth: `listing.Vocabulary` in `internal/listing/listing.go`.

| `type` | allowed `value`s |
| --- | --- |
| `natural_light` | `high` `medium` `low` |
| `noise_level` | `quiet` `moderate` `noisy` |
| `orientation` | `norte` `sur` `este` `oeste` `noreste` `noroeste` `sudeste` `sudoeste` |
| `exposure` | `frente` `contrafrente` `interno` `lateral` |
| `outdoor_space` | `balcon` `balcon_terraza` `terraza` `patio` `jardin` `none` |
| `condition` | `a_estrenar` `excelente` `muy_bueno` `bueno` `a_refaccionar` |
| `furnished` | `yes` `no` |
| `pets_allowed` | `yes` `no` |
| `air_conditioning` | `yes` `no` |
| `heating` | `central` `individual` `losa_radiante` `none` |
| `suitable_for` | `vivienda` `profesional` `oficina` `comercial` `estudiantes` |
| `amenity` | `pileta` `gimnasio` `laundry` `coworking` `sum` `seguridad` `parrilla` `ascensor` `cochera` `solarium` `terraza_comun` |
| `transit_access` | `subte_a` `subte_b` `subte_c` `subte_d` `subte_e` `subte_h` `tren` `colectivo` |

Keys are English to match the codebase. Values stay Spanish where the Argentine
term carries meaning a translation would lose — `contrafrente` is not
"rear-facing", a `parrilla` is not a barbecue.

**Adding a term is a schema change in spirit.** Both sides of the marketplace
must speak the same words or nothing matches, so extend `listing.Vocabulary`
and re-run the parse; do not let a tool invent values at query time.

## Three traps for a tool implementer

**1. NULL means "not published", never zero.** A listing with
`expenses_amount IS NULL` did not disclose its expensas. Coalescing that to 0
makes it the cheapest result in every search — the single most damaging bug
available in this schema. Filters on a nullable column must decide explicitly
whether unpublished rows are included, and say so to the user.

**2. Currencies do not compare.** `price_currency` is `ARS` or `USD` and there
is no stored exchange rate. `WHERE price_amount < 1000` silently mixes a
USD 1.000 flat with an ARS 1.000 one. Always constrain currency, or convert
with a rate the caller supplies.

**3. `provenance` is the trust signal — surface it.** `stated` means the
listing says it outright; `inferred` means a model concluded it. `exposure` is
special: it is always `stated` because ZonaProp publishes the disposition as
its own field, so it is read, not inferred. A buyer agent should weigh a
`stated` claim with `evidence` differently from a bare `inferred` one, and
should be able to quote the evidence back to the user.

## Suggested tool shape

Hard constraints go to SQL, preferences go to ranking. Do not ask a model to
filter on price.

```sql
SELECT l.url, l.neighborhood, a.name AS agency,
       l.price_amount, l.price_currency, l.expenses_amount,
       l.rooms, l.bedrooms, l.total_area_m2,
       l.description,
       COALESCE(
         json_agg(json_build_object('type', at.type, 'value', at.value,
                                    'provenance', at.provenance, 'evidence', at.evidence))
         FILTER (WHERE at.type IS NOT NULL), '[]') AS attributes
FROM listings l
LEFT JOIN agencies a ON a.id = l.agency_id
LEFT JOIN listing_attributes at ON at.listing_id = l.id
WHERE l.neighborhood = ANY($1)
  AND l.operation = 'alquiler'
  AND (l.price_amount IS NULL OR (l.price_currency = $2 AND l.price_amount <= $3))
  AND (l.bedrooms IS NULL OR l.bedrooms >= $4)
GROUP BY l.id, a.name;
```

Then rank in the agent using the attributes and the description — that is where
the trade-off reasoning belongs, and the only part of this that needs a model.

To require an attribute rather than merely prefer it:

```sql
AND EXISTS (SELECT 1 FROM listing_attributes r
            WHERE r.listing_id = l.id AND r.type = 'natural_light' AND r.value = 'high')
```
