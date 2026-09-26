-- +goose Up
CREATE TABLE todos (
    id            uuid PRIMARY KEY,
    list_id       uuid NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title         text NOT NULL,
    notes         text,
    status        text NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'done')),
    priority      integer NOT NULL DEFAULT 0,
    due_at        timestamptz,
    completed_at  timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX todos_user_status_due_idx ON todos(user_id, status, due_at);
CREATE INDEX todos_list_created_id_idx ON todos(list_id, created_at DESC, id DESC);

-- +goose Down
DROP TABLE todos;
