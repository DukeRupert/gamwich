-- +goose Up
DELETE FROM settings WHERE key IN ('email_postmark_token', 'email_from_address', 'email_base_url');

-- +goose Down
INSERT OR IGNORE INTO settings (household_id, key, value)
SELECT id, 'email_postmark_token', '' FROM households;

INSERT OR IGNORE INTO settings (household_id, key, value)
SELECT id, 'email_from_address', '' FROM households;

INSERT OR IGNORE INTO settings (household_id, key, value)
SELECT id, 'email_base_url', '' FROM households;
