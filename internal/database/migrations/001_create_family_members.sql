-- +goose Up
CREATE TABLE family_members (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    color TEXT NOT NULL DEFAULT '#3B82F6',
    avatar_emoji TEXT NOT NULL DEFAULT '😀',
    pin TEXT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- +goose StatementBegin
CREATE TRIGGER update_family_members_updated_at
AFTER UPDATE ON family_members
FOR EACH ROW
BEGIN
    UPDATE family_members SET updated_at = datetime('now') WHERE id = OLD.id;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS update_family_members_updated_at;
DROP TABLE IF EXISTS family_members;
