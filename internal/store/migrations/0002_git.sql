-- +goose Up
-- +goose StatementBegin

ALTER TABLE apps ADD COLUMN git_url TEXT NOT NULL DEFAULT '';
ALTER TABLE apps ADD COLUMN git_branch TEXT NOT NULL DEFAULT '';
ALTER TABLE apps ADD COLUMN git_dockerfile TEXT NOT NULL DEFAULT '';
ALTER TABLE apps ADD COLUMN git_token_encrypted BLOB;
ALTER TABLE apps ADD COLUMN webhook_secret_encrypted BLOB;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- SQLite cannot DROP COLUMN before 3.35; leave them in place on down.
SELECT 1;
-- +goose StatementEnd
