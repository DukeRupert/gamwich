-- +goose Up
CREATE TABLE waitlist (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL,
    plan TEXT NOT NULL DEFAULT 'hosted',
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE(email, plan)
);

-- +goose Down
DROP TABLE IF EXISTS waitlist;
