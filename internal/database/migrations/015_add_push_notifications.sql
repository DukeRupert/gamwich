-- +goose Up

-- Push subscriptions (Web Push API subscriptions per device)
CREATE TABLE push_subscriptions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    household_id INTEGER NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    endpoint TEXT NOT NULL UNIQUE,
    p256dh_key TEXT NOT NULL,
    auth_key TEXT NOT NULL,
    device_name TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_push_subscriptions_user ON push_subscriptions(user_id, household_id);
CREATE INDEX idx_push_subscriptions_household ON push_subscriptions(household_id);

-- Notification preferences (per-user, per-household, per-type)
CREATE TABLE notification_preferences (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    household_id INTEGER NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    notification_type TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE(user_id, household_id, notification_type)
);

-- +goose StatementBegin
CREATE TRIGGER trg_notification_preferences_updated_at
AFTER UPDATE ON notification_preferences
FOR EACH ROW
BEGIN
    UPDATE notification_preferences SET updated_at = datetime('now') WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- Sent notifications dedup table
CREATE TABLE sent_notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    household_id INTEGER NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    notification_type TEXT NOT NULL,
    reference_id TEXT NOT NULL,
    lead_time_minutes INTEGER NOT NULL DEFAULT 0,
    sent_at DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE(household_id, notification_type, reference_id, lead_time_minutes)
);

CREATE INDEX idx_sent_notifications_sent_at ON sent_notifications(sent_at);

-- Add reminder_minutes to calendar_events
ALTER TABLE calendar_events ADD COLUMN reminder_minutes INTEGER;

-- +goose Down

ALTER TABLE calendar_events DROP COLUMN reminder_minutes;
DROP TABLE IF EXISTS sent_notifications;
DROP TRIGGER IF EXISTS trg_notification_preferences_updated_at;
DROP TABLE IF EXISTS notification_preferences;
DROP TABLE IF EXISTS push_subscriptions;
