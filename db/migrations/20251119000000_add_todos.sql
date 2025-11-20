-- +goose Up
-- +goose StatementBegin
CREATE TABLE todos (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    content TEXT NOT NULL,
    active_form TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('pending', 'in_progress', 'completed')),
    position INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    completed_at INTEGER,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE INDEX idx_todos_session_id ON todos(session_id);
CREATE INDEX idx_todos_status ON todos(status);
CREATE INDEX idx_todos_session_status ON todos(session_id, status);
CREATE INDEX idx_todos_position ON todos(session_id, position);

-- Trigger to auto-update updated_at
CREATE TRIGGER todos_updated_at_trigger
AFTER UPDATE ON todos
FOR EACH ROW
BEGIN
    UPDATE todos SET updated_at = (strftime('%s', 'now') * 1000) WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS todos_updated_at_trigger;
DROP INDEX IF EXISTS idx_todos_position;
DROP INDEX IF EXISTS idx_todos_session_status;
DROP INDEX IF EXISTS idx_todos_status;
DROP INDEX IF EXISTS idx_todos_session_id;
DROP TABLE IF EXISTS todos;
-- +goose StatementEnd
