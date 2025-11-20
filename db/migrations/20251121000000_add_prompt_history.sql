-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS prompt_history (
    id TEXT PRIMARY KEY,
    prompt TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    UNIQUE(prompt)
);

CREATE INDEX IF NOT EXISTS idx_prompt_history_created_at
ON prompt_history(created_at DESC);

-- Trigger to enforce history limit (1000 prompts)
CREATE TRIGGER IF NOT EXISTS prompt_history_limit_trigger
AFTER INSERT ON prompt_history
WHEN (SELECT COUNT(*) FROM prompt_history) > 1000
BEGIN
    DELETE FROM prompt_history
    WHERE id IN (
        SELECT id FROM prompt_history
        ORDER BY created_at ASC
        LIMIT ((SELECT COUNT(*) FROM prompt_history) - 1000)
    );
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS prompt_history_limit_trigger;
DROP INDEX IF EXISTS idx_prompt_history_created_at;
DROP TABLE IF EXISTS prompt_history;
-- +goose StatementEnd
