-- Eligibility requirements and qualification (docs/adr/0001-eligibility-rules-as-data.md).
-- Fact names and their choices are rows, never enums (idea.md §8).

CREATE TABLE eligibility_facts (
    name       text PRIMARY KEY,
    -- Rules and qualifications may only reference admissible facts. Protected
    -- characteristics are listed explicitly as not admissible.
    admissible boolean NOT NULL,
    -- What the qualification form offers for this fact.
    choices    jsonb NOT NULL DEFAULT '[]'
);

INSERT INTO eligibility_facts (name, admissible, choices) VALUES
    ('guarantee',      true,  '["propietaria", "caucion"]'),
    ('income_band',    true,  '["0-1000000", "1000000-2000000", "2000000-3000000", "3000000-"]'),
    ('caucion_quoted', true,  '["yes", "no"]'),
    ('age',            false, '[]'),
    ('nationality',    false, '[]'),
    ('gender',         false, '[]');

CREATE TABLE listing_eligibility_rules (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    listing_id bigint NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    fact       text NOT NULL REFERENCES eligibility_facts(name),
    operator   text NOT NULL,
    "values"   jsonb NOT NULL,
    hardness   text NOT NULL CHECK (hardness IN ('hard', 'discretionary')),
    visibility text NOT NULL DEFAULT 'public' CHECK (visibility IN ('public', 'private')),
    source     text NOT NULL DEFAULT 'parsed' CHECK (source IN ('parsed', 'declared', 'observed')),
    evidence   text NOT NULL DEFAULT ''
);
CREATE INDEX listing_eligibility_rules_listing ON listing_eligibility_rules (listing_id);

CREATE TABLE user_qualifications (
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    fact    text NOT NULL REFERENCES eligibility_facts(name),
    value   text NOT NULL,
    PRIMARY KEY (user_id, fact, value)
);
