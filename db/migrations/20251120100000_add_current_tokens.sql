-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN current_input_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN current_output_tokens INTEGER NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN current_input_tokens;
ALTER TABLE sessions DROP COLUMN current_output_tokens;
-- +goose StatementEnd
