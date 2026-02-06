-- +goose Up
ALTER TABLE calendar_events ADD COLUMN recurrence_rule TEXT NOT NULL DEFAULT '';
ALTER TABLE calendar_events ADD COLUMN recurrence_parent_id INTEGER REFERENCES calendar_events(id) ON DELETE CASCADE;
ALTER TABLE calendar_events ADD COLUMN original_start_time DATETIME;
ALTER TABLE calendar_events ADD COLUMN cancelled INTEGER NOT NULL DEFAULT 0;
CREATE INDEX idx_calendar_events_recurrence_parent ON calendar_events(recurrence_parent_id);

-- +goose Down
DROP INDEX IF EXISTS idx_calendar_events_recurrence_parent;
-- SQLite doesn't support DROP COLUMN before 3.35.0; recreate table
CREATE TABLE calendar_events_backup AS SELECT id, title, description, start_time, end_time, all_day, family_member_id, location, created_at, updated_at FROM calendar_events;
DROP TABLE calendar_events;
CREATE TABLE calendar_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    start_time DATETIME NOT NULL,
    end_time DATETIME NOT NULL,
    all_day INTEGER NOT NULL DEFAULT 0,
    family_member_id INTEGER REFERENCES family_members(id) ON DELETE SET NULL,
    location TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
INSERT INTO calendar_events SELECT * FROM calendar_events_backup;
DROP TABLE calendar_events_backup;
CREATE INDEX idx_calendar_events_start_time ON calendar_events(start_time);
CREATE INDEX idx_calendar_events_family_member ON calendar_events(family_member_id);
-- +goose StatementBegin
CREATE TRIGGER update_calendar_events_updated_at
AFTER UPDATE ON calendar_events
FOR EACH ROW
BEGIN
    UPDATE calendar_events SET updated_at = datetime('now') WHERE id = OLD.id;
END;
-- +goose StatementEnd
