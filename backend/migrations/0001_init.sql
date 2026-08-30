-- Interviewer accounts, their login sessions, and the rooms they own.
--
-- Two kinds of secret live here and they are hashed differently on purpose:
--
--   * A password is chosen by a human, so it is low-entropy and must be slow
--     to guess. It goes through bcrypt, in the application.
--   * A session or invite token is 32 bytes from crypto/rand. Brute force is
--     already impossible, so the only job of hashing is to make a leaked
--     database dump useless. SHA-256 is right for that, and being fast means
--     it does not tax every authenticated request.
--
-- Either way the plaintext is never stored: the server hashes what it is
-- given and looks up the hash.

CREATE TABLE users (
    id            BIGSERIAL   PRIMARY KEY,
    email         TEXT        NOT NULL,
    password_hash TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Case-insensitive uniqueness without the citext extension, which would have
-- to be installed in every environment. Registration and login both look up
-- by lower(email), so this index serves the query as well as the constraint.
CREATE UNIQUE INDEX users_email_lower_key ON users (lower(email));

CREATE TABLE sessions (
    token_hash BYTEA       PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);

-- Logging out one device should not log out the others, so sessions are
-- deleted individually; this index is for the sweep of expired rows and for
-- "log out everywhere".
CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

CREATE TABLE rooms (
    -- Not a serial: the ID is in the URL both people paste to each other, and
    -- a guessable sequence would let anyone walk the list of live interviews.
    -- The application generates it and it must satisfy the same pattern the
    -- WebSocket handler enforces, ^[A-Za-z0-9_-]{1,64}$.
    id                TEXT        PRIMARY KEY
                      CHECK (id ~ '^[A-Za-z0-9_-]{1,64}$'),
    owner_id          BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title             TEXT        NOT NULL DEFAULT '',
    -- The candidate's half of the shareable link. Rotatable without changing
    -- the room ID, so a link sent to the wrong address can be revoked without
    -- disturbing anyone already in the room.
    invite_token_hash BYTEA       NOT NULL,
    language          TEXT        NOT NULL DEFAULT 'python',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Set when the interviewer ends the session. A closed room refuses new
    -- joins but is kept, because it is the record that the interview happened.
    closed_at         TIMESTAMPTZ
);

CREATE INDEX rooms_owner_id_created_at_idx ON rooms (owner_id, created_at DESC);
