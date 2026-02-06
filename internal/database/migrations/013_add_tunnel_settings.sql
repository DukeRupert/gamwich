-- +goose Up
-- Seed tunnel settings for existing households
INSERT OR IGNORE INTO settings (household_id, key, value)
SELECT id, 'tunnel_enabled', 'false' FROM households;

INSERT OR IGNORE INTO settings (household_id, key, value)
SELECT id, 'tunnel_token', '' FROM households;

-- +goose Down
DELETE FROM settings WHERE key IN ('tunnel_enabled', 'tunnel_token');
