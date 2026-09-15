-- Accounts for people who sign in: searchers whose agent remembers them, and
-- realtors who manage properties. Searching itself never needs a row here.

CREATE TABLE users (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- Stored normalized so uniqueness cannot be dodged by changing case.
    email         text NOT NULL UNIQUE CHECK (email = lower(email)),
    name          text NOT NULL,
    role          text NOT NULL CHECK (role IN ('searcher', 'realtor')),
    -- pbkdf2-sha256$<iterations>$<salt>$<key>; never the password itself.
    password_hash text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    -- SHA-256 of the bearer token. The token itself is only ever held by the
    -- client, so reading this table does not let anyone sign in as a user.
    token_hash bytea PRIMARY KEY,
    user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);
