-- Realtor ownership is separate from the source agency name. The former is
-- authorization state; the latter is listing content shown to buyers.
ALTER TABLE listings
    ADD COLUMN owner_user_id bigint REFERENCES users (id),
    ADD COLUMN catalog_status text NOT NULL DEFAULT 'active'
        CHECK (catalog_status IN ('active', 'archived'));

CREATE INDEX listings_owner_catalog_idx
    ON listings (owner_user_id, catalog_status);

-- Contact totals are derived from immutable, idempotent events. The browser
-- never sends a counter value, and a retried intent cannot increment twice.
CREATE TABLE listing_contact_intents (
    intent_id  uuid PRIMARY KEY,
    listing_id bigint NOT NULL REFERENCES listings (id) ON DELETE CASCADE,
    source     text NOT NULL CHECK (source IN ('search_result_card')),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX listing_contact_intents_listing_idx
    ON listing_contact_intents (listing_id);
