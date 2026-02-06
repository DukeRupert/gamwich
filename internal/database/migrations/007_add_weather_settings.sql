-- +goose Up
INSERT OR IGNORE INTO settings (key, value) VALUES
    ('weather_latitude', ''),
    ('weather_longitude', ''),
    ('weather_units', 'fahrenheit');

-- +goose Down
DELETE FROM settings WHERE key IN ('weather_latitude', 'weather_longitude', 'weather_units');
