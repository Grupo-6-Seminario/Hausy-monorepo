-- Existing rows remain visible until the one-time audit has reviewed them.
-- New and changed rows are pending and cannot enter buyer search.
ALTER TABLE listings ADD COLUMN quality_status text NOT NULL DEFAULT 'legacy'
    CHECK (quality_status IN ('legacy', 'pending', 'passed', 'withheld'));
ALTER TABLE listings ADD COLUMN quality_evidence jsonb;
ALTER TABLE listings ADD COLUMN quality_checked_at timestamptz;
CREATE INDEX listings_quality_status_idx ON listings (quality_status);
