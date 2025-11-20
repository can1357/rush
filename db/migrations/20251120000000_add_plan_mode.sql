-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN plan_mode INTEGER NOT NULL DEFAULT 0 CHECK (plan_mode IN (0, 1));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN plan_mode;
-- +goose StatementEnd
