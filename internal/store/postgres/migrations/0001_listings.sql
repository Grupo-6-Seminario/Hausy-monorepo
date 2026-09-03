-- Listing store for the seller-side inventory.
--
-- The split between columns and rows is deliberate and follows AGENTS.md:
-- hard constraints a buyer states numerically (price, expenses, area, rooms)
-- are typed columns so they are filtered with ordinary SQL, while the fuzzy
-- qualities that only exist in listing prose live in listing_attributes as
-- (type, value) pairs -- the same shape the buyer agent already emits from a
-- natural-language query, so the two sides join directly.

CREATE TABLE agencies (
    id   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name text NOT NULL UNIQUE
);

CREATE TABLE listings (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source       text NOT NULL DEFAULT 'zonaprop',
    url          text NOT NULL UNIQUE,
    neighborhood text NOT NULL,
    agency_id    bigint REFERENCES agencies (id),
    address      text,
    description  text NOT NULL,

    -- Deterministic facts read straight from the listing page. Nullable
    -- because a listing may simply not publish them; NULL means "not
    -- published", never "zero".
    operation        text,
    price_amount     numeric(14, 2),
    price_currency   text,
    expenses_amount  numeric(14, 2),
    expenses_currency text,
    total_area_m2    numeric(10, 2),
    covered_area_m2  numeric(10, 2),
    rooms            integer,
    bedrooms         integer,
    bathrooms        integer,
    parking_spaces   integer,
    age_years        integer,
    floor            text,

    -- Provenance. parser_model records which model produced the rows in
    -- listing_attributes, so a re-parse with a different model is visible
    -- rather than silently mixed in with the old one.
    scraped_at   timestamptz NOT NULL,
    parsed_at    timestamptz,
    parser_model text,
    ingested_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX listings_neighborhood_idx ON listings (neighborhood);
CREATE INDEX listings_price_idx ON listings (price_currency, price_amount);
CREATE INDEX listings_bedrooms_idx ON listings (bedrooms);

CREATE TABLE listing_attributes (
    listing_id bigint NOT NULL REFERENCES listings (id) ON DELETE CASCADE,
    type       text NOT NULL,
    value      text NOT NULL,

    -- The trust distinction AGENTS.md calls for: 'stated' means the listing
    -- says it in so many words, 'inferred' means the parser concluded it from
    -- surrounding prose. A buyer agent weighs the two differently.
    provenance text NOT NULL CHECK (provenance IN ('stated', 'inferred')),

    -- The phrase the value was drawn from, kept verbatim so a claim can be
    -- traced back to the seller's own words instead of being taken on faith.
    evidence   text,

    PRIMARY KEY (listing_id, type, value)
);

CREATE INDEX listing_attributes_lookup_idx ON listing_attributes (type, value);
