-- +goose Up
ALTER TABLE chore_completions ADD COLUMN points_earned INTEGER NOT NULL DEFAULT 0;

CREATE TABLE rewards (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    point_cost INTEGER NOT NULL DEFAULT 0,
    active INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE reward_redemptions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    reward_id INTEGER NOT NULL REFERENCES rewards(id) ON DELETE CASCADE,
    redeemed_by INTEGER REFERENCES family_members(id) ON DELETE SET NULL,
    points_spent INTEGER NOT NULL DEFAULT 0,
    redeemed_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_reward_redemptions_reward_id ON reward_redemptions(reward_id);
CREATE INDEX idx_reward_redemptions_redeemed_by ON reward_redemptions(redeemed_by);

-- Seed leaderboard setting
INSERT OR IGNORE INTO settings (key, value) VALUES ('rewards_leaderboard_enabled', 'true');

-- +goose Down
DROP INDEX IF EXISTS idx_reward_redemptions_redeemed_by;
DROP INDEX IF EXISTS idx_reward_redemptions_reward_id;
DROP TABLE IF EXISTS reward_redemptions;
DROP TABLE IF EXISTS rewards;

-- SQLite does not support DROP COLUMN, so we recreate the table
CREATE TABLE chore_completions_backup (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chore_id INTEGER NOT NULL REFERENCES chores(id) ON DELETE CASCADE,
    completed_by INTEGER REFERENCES family_members(id) ON DELETE SET NULL,
    completed_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
INSERT INTO chore_completions_backup (id, chore_id, completed_by, completed_at)
    SELECT id, chore_id, completed_by, completed_at FROM chore_completions;
DROP TABLE chore_completions;
ALTER TABLE chore_completions_backup RENAME TO chore_completions;
CREATE INDEX idx_chore_completions_chore_id ON chore_completions(chore_id);
CREATE INDEX idx_chore_completions_completed_by ON chore_completions(completed_by);
CREATE INDEX idx_chore_completions_completed_at ON chore_completions(completed_at);

DELETE FROM settings WHERE key = 'rewards_leaderboard_enabled';
