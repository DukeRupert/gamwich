-- +goose Up
UPDATE settings SET value = 'light' WHERE key = 'theme_selected' AND value = 'garden';
UPDATE settings SET value = 'light' WHERE key = 'theme_light' AND value = 'garden';
UPDATE settings SET value = 'dark' WHERE key = 'theme_dark' AND value = 'forest';
UPDATE settings SET value = 'light' WHERE key = 'theme_selected' AND value = 'gamwich';
UPDATE settings SET value = 'light' WHERE key = 'theme_selected' AND value = 'cupcake';
UPDATE settings SET value = 'dark' WHERE key = 'theme_selected' AND value = 'dracula';
UPDATE settings SET value = 'light' WHERE key = 'theme_light' AND value = 'cupcake';
UPDATE settings SET value = 'light' WHERE key = 'theme_light' AND value = 'gamwich';
UPDATE settings SET value = 'dark' WHERE key = 'theme_dark' AND value = 'dracula';
UPDATE settings SET value = 'dark' WHERE key = 'theme_dark' AND value = 'gamwich';

-- +goose Down
UPDATE settings SET value = 'garden' WHERE key = 'theme_selected' AND value = 'light';
UPDATE settings SET value = 'garden' WHERE key = 'theme_light' AND value = 'light';
UPDATE settings SET value = 'forest' WHERE key = 'theme_dark' AND value = 'dark';
