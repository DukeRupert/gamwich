-- +goose Up

-- Backups table
CREATE TABLE IF NOT EXISTS backups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    household_id INTEGER NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    s3_key TEXT NOT NULL,
    size_bytes INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'uploading', 'completed', 'failed')),
    error_message TEXT,
    started_at DATETIME,
    completed_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_backups_household_id ON backups(household_id);
CREATE INDEX idx_backups_status ON backups(status);
CREATE INDEX idx_backups_created_at ON backups(created_at);

-- +goose StatementBegin
CREATE TRIGGER trg_backups_updated_at
AFTER UPDATE ON backups
FOR EACH ROW
BEGIN
    UPDATE backups SET updated_at = datetime('now') WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- Seed backup settings for existing households
INSERT OR IGNORE INTO settings (household_id, key, value)
SELECT id, 'backup_enabled', 'false' FROM households;

INSERT OR IGNORE INTO settings (household_id, key, value)
SELECT id, 'backup_schedule_hour', '3' FROM households;

INSERT OR IGNORE INTO settings (household_id, key, value)
SELECT id, 'backup_retention_days', '30' FROM households;

INSERT OR IGNORE INTO settings (household_id, key, value)
SELECT id, 'backup_passphrase_salt', '' FROM households;

INSERT OR IGNORE INTO settings (household_id, key, value)
SELECT id, 'backup_passphrase_hash', '' FROM households;

-- +goose Down
DROP TRIGGER IF EXISTS trg_backups_updated_at;
DROP TABLE IF EXISTS backups;
DELETE FROM settings WHERE key IN ('backup_enabled', 'backup_schedule_hour', 'backup_retention_days', 'backup_passphrase_salt', 'backup_passphrase_hash');
