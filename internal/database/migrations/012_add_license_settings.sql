-- +goose Up
-- Seed license_key setting for existing households
INSERT OR IGNORE INTO settings (household_id, key, value)
SELECT id, 'license_key', '' FROM households;

-- +goose Down
DELETE FROM settings WHERE key = 'license_key';
