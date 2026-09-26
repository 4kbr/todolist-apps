-- +goose Up
-- +goose StatementBegin
CREATE TABLE
  sessions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    refresh_token_hash bytea NOT NULL UNIQUE,
    user_agent text,
    ip inet,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now ()
  );

CREATE INDEX sessions_user_id_idx ON sessions (user_id);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE sessions;

-- +goose StatementEnd