-- +goose Up
CREATE TABLE notes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    author_id INTEGER REFERENCES family_members(id) ON DELETE SET NULL,
    pinned INTEGER NOT NULL DEFAULT 0,
    priority TEXT NOT NULL DEFAULT 'normal',
    expires_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_notes_author_id ON notes(author_id);
CREATE INDEX idx_notes_pinned ON notes(pinned);
CREATE INDEX idx_notes_expires_at ON notes(expires_at);

-- +goose StatementBegin
CREATE TRIGGER update_notes_updated_at
AFTER UPDATE ON notes
FOR EACH ROW
BEGIN
    UPDATE notes SET updated_at = datetime('now') WHERE id = OLD.id;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS update_notes_updated_at;
DROP TABLE IF EXISTS notes;
