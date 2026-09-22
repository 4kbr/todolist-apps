-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE
  users (
    id uuid PRIMARY KEY,
    email citext NOT NULL UNIQUE,
    password_hash text NOT NULL,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT NOW (),
    updated_at timestamptz NOT NULL DEFAULT NOW ()
  );

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE users;

DROP EXTENSION IF EXISTS citext;

-- +goose StatementEnd