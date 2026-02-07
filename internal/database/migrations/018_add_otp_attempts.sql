-- +goose Up
ALTER TABLE magic_links ADD COLUMN attempts INTEGER NOT NULL DEFAULT 0;
CREATE INDEX idx_magic_links_email_token ON magic_links (email, token);

-- +goose Down
DROP INDEX IF EXISTS idx_magic_links_email_token;
ALTER TABLE magic_links DROP COLUMN attempts;
