-- +goose Up
INSERT OR IGNORE INTO settings (key, value) VALUES
    ('theme_mode', 'manual'),
    ('theme_selected', 'garden'),
    ('theme_light', 'garden'),
    ('theme_dark', 'forest');

-- +goose Down
DELETE FROM settings WHERE key IN ('theme_mode', 'theme_selected', 'theme_light', 'theme_dark');
