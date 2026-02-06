-- +goose Up
CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Seed default kiosk settings
INSERT OR IGNORE INTO settings (key, value) VALUES
    ('idle_timeout_minutes', '5'),
    ('quiet_hours_enabled', 'false'),
    ('quiet_hours_start', '22:00'),
    ('quiet_hours_end', '06:00'),
    ('burn_in_prevention', 'true');

-- +goose Down
DROP TABLE IF EXISTS settings;
