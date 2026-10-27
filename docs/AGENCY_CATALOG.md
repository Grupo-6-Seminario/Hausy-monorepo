# Agency catalog contract

The realtor UI and any future machine-to-machine transport share the typed
application boundary in `internal/agency`. HTTP authenticates and translates;
it does not own catalog rules. Postgres persists the interface; the in-memory
adapter stands in for it in tests.

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
operation, price, expenses, area, room counts, parking, age, floor, and its
declared eligibility requirements. `id`, `agency`, `source`, ownership,
catalog status, `contact_count`, and `inquiry_count` are server-owned.

An agency's property is published in buyer search when saved, without the
quality review scraped listings wait for; a review that withholds it still
hides it, including after an agency edit. Only a new quality review can release
that hold.

### Declared requirements

`requirements` replaces the property's whole set of eligibility requirements
(CONTEXT.md, Eligibility requirement). Each is `{fact, operator, values,
hardness}`, with `hardness` `hard` or `discretionary`; the server adds
`source: "declared"`, `visibility: "public"`, and `evidence`, its Spanish
wording, which the result card shows as the agency's. Only these are accepted
(`agency.PrepareRequirements`), at most one per fact:

| Fact | Operator | Values |
| --- | --- | --- |
| `guarantee` | `one_of` | accepted catalog choices, never `none` |
| `income_band` | `income_multiple`, `income_multiple_with_expenses` | one number of rents, e.g. `"3"` |
| `income_documented` | `one_of` | `["yes"]` |
| `pets` | `subset_of` | admitted choices; `none` is always added |

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

## Inquiry API

A searcher sends an inquiry about a property an agency manages in Hausy
(`managed: true` on the search result) without an account (CONTEXT.md,
Inquiry). The server builds the preview and the saved inquiry from the same
input. Send includes the revision returned by preview:

```http
POST /api/listings/{listingId}/inquiries/preview
POST /api/listings/{listingId}/inquiries
Content-Type: application/json

{
  "inquiry_id": "6f1c2a1e-3b4d-4e5f-8a9b-0c1d2e3f4a5b",
  "preview_revision": "<revision from preview>",
  "name": "Ana",
  "email": "ana@example.com",
  "phone": "",
  "message": "¿Se puede visitar el sábado?",
  "searching": ["2 ambientes", "Palermo"],
  "qualification": {"guarantee": ["caucion"], "income_band": ["2000000-3000000"]}
}
```

- The server assesses the qualification against the property's requirements.
  The inquiry carries one row per requirement (its wording, the labels of what
  the searcher declared, the outcome) and, of the qualification, only the
  facts those requirements name.
- `searching` holds up to 50 search phrases without the budget. The client
  drops price bounds before sending. Preview and send enforce the same limit.
- The client uses the search response's `qualification`, which includes form,
  chat, and clarification answers used to assess the displayed cards.
- Preview answers `200`, includes `preview_revision`, and stores nothing.
  Send needs that revision, `inquiry_id`, a name, and an email or a phone.
  Phone numbers start with `+` and the country code and are stored as `+`
  followed by digits. Local numbers without a country code are rejected.
- Send answers `201`, or `200` with the stored inquiry when retried with the
  same id, including after the listing changes or is archived.
- A changed preview returns `409 preview_changed` without storing a new
  inquiry. The browser reloads the preview, keeps contact details, and asks
  the searcher to send again.
- `422 ineligible` when a strict requirement rules the searcher out, `404` for
  a property no agency manages in Hausy, `409` when the id belongs to another
  property, `400` with `field` for invalid input.

The managing realtor reads them, newest first:

| Method and path | Result |
| --- | --- |
| `GET /api/agency/inquiries` | `200 {"inquiries": [...]}` for the authenticated owner's properties |

## Current scope boundary

One realtor account is one catalog owner in this first slice. Shared agency
organizations, multiple staff memberships, media uploads, private or observed
requirements, notifying the agency outside the panel, letting a searcher edit
or withdraw a sent inquiry, and unique-person analytics are not modeled yet. Adding an
organization-to-owner identity adapter later should not change the browser
payloads or `agency.Catalog`; adding those product capabilities still requires
their own explicit data contracts.
