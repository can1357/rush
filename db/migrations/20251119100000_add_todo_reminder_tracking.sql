-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN last_todo_write_turn INTEGER NOT NULL DEFAULT 0 CHECK (last_todo_write_turn >= 0);
ALTER TABLE sessions ADD COLUMN last_reminder_turn INTEGER NOT NULL DEFAULT 0 CHECK (last_reminder_turn >= 0);
ALTER TABLE sessions ADD COLUMN assistant_turn_count INTEGER NOT NULL DEFAULT 0 CHECK (assistant_turn_count >= 0);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN last_todo_write_turn;
ALTER TABLE sessions DROP COLUMN last_reminder_turn;
ALTER TABLE sessions DROP COLUMN assistant_turn_count;
-- +goose StatementEnd
