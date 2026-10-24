-- +goose Up
-- When a person who signed up themselves agreed to the processing of their data (NULL for demo accounts).
ALTER TABLE users ADD COLUMN consented_at timestamptz;

-- +goose Down
ALTER TABLE users DROP COLUMN consented_at;
