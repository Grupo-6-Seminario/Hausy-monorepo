# Agency catalog contract

The realtor UI and any future machine-to-machine transport share the typed
application boundary in `internal/agency`. HTTP authenticates and translates;
it does not own catalog rules. Postgres persists the interface, while the
in-memory adapter keeps local development functional without a database.

Current flow:

```text
realtor browser → same-origin Next proxy → authenticated HTTP adapter
                                             ↓
                                      agency.Catalog
                                             ↓
                                  Postgres or memory adapter
```

A future A2A adapter belongs beside HTTP and calls `agency.Catalog`. It should
not bypass the interface, talk to Postgres directly, or introduce free-text
agent chat for deterministic mutations. This keeps the unresolved per-agency
agent versus shared-policy decision open.

## Realtor catalog API

All catalog routes require a realtor bearer session. The server derives the
owner from that session; the browser never sends an owner, agency id, role, or
contact total.

| Method and path | Result |
| --- | --- |
| `GET /api/agency/catalog` | `200 {"properties": [...]}` for the authenticated owner |
| `POST /api/agency/catalog` | validates editable facts and returns the property with `201` |
| `PATCH /api/agency/catalog/{listingId}` | replaces editable facts on one owned active property and returns it with `200` |
| `DELETE /api/agency/catalog/{listingId}` | archives one owned active property and returns `204` |

An editable property contains the canonical deterministic fields from
`internal/agency.PropertyInput`: URL, neighborhood, address, description,
operation, price, expenses, area, room counts, parking, age, and floor.
`id`, `agency`, `source`, ownership, catalog status, and `contact_count` are
server-owned.

## Contact-intent API

The buyer card sends one event before opening its contact destination:

```http
POST /api/listings/{listingId}/contact-intents
Content-Type: application/json

{
  "intent_id": "4c065799-5ad0-4df4-9b09-8dcc541507d2",
  "source": "search_result_card"
}
```

Contract:

- `listingId` is the canonical result id already returned by search. It is not
  parsed from the external listing URL.
- `intent_id` is generated once with `crypto.randomUUID()` for a deliberate
  activation and reused only when retrying that same activation.
- `source` is the closed enum `search_result_card`.
- The browser sends no counter, owner, agency, user id, or timestamp.
- The first accepted event returns `201`; an idempotent replay returns `200`.
  Both bodies are `{"intent_id":"...","listing_id":"...","recorded":true}`.
- Reusing an intent id for another listing returns `409`. Invalid input returns
  `400`; a missing or archived listing returns `404`.
- Tracking is best effort from the buyer experience: a failed analytics write
  must not prevent the user from reaching the contact destination.

The aggregate is an event count. It is deliberately labeled “contactos
iniciados” because this contract does not prove that a message was delivered
or identify unique people.

## Current scope boundary

One realtor account is one catalog owner in this first slice. Shared agency
organizations, multiple staff memberships, media uploads, eligibility-rule
editing, and unique-person analytics are not modeled yet. Adding an
organization-to-owner identity adapter later should not change the browser
payloads or `agency.Catalog`; adding those product capabilities still requires
their own explicit data contracts.
