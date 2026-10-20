-- +goose Up
-- Expected-loss figures of the request (PD, LGD, EAD, EL, premium) stored with the score for audit.
ALTER TABLE scoring_results ADD COLUMN metrics jsonb NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE scoring_results DROP COLUMN metrics;
