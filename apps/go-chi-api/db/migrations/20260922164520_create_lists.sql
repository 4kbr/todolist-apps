-- +goose Up
-- +goose StatementBegin
CREATE TABLE
  lists (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name text NOT NULL,
    position integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now (),
    updated_at timestamptz NOT NULL DEFAULT now (),
    UNIQUE (user_id, name)
  );

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE lists;

-- +goose StatementEnd