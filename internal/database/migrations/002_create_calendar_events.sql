-- +goose Up
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

-- +goose Down
DROP TRIGGER IF EXISTS update_calendar_events_updated_at;
DROP TABLE IF EXISTS calendar_events;
