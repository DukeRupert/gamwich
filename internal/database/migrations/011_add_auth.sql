-- +goose Up

-- New auth tables
CREATE TABLE households (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- +goose StatementBegin
CREATE TRIGGER update_households_updated_at
AFTER UPDATE ON households
FOR EACH ROW
BEGIN
    UPDATE households SET updated_at = datetime('now') WHERE id = OLD.id;
END;
-- +goose StatementEnd

-- Seed default household
INSERT INTO households (name) VALUES ('My Household');

CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- +goose StatementBegin
CREATE TRIGGER update_users_updated_at
AFTER UPDATE ON users
FOR EACH ROW
BEGIN
    UPDATE users SET updated_at = datetime('now') WHERE id = OLD.id;
END;
-- +goose StatementEnd

CREATE TABLE household_members (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    household_id INTEGER NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member',
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE(household_id, user_id)
);

-- +goose StatementBegin
CREATE TRIGGER update_household_members_updated_at
AFTER UPDATE ON household_members
FOR EACH ROW
BEGIN
    UPDATE household_members SET updated_at = datetime('now') WHERE id = OLD.id;
END;
-- +goose StatementEnd

CREATE TABLE sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token TEXT NOT NULL UNIQUE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    household_id INTEGER NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_sessions_token ON sessions(token);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE magic_links (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL,
    purpose TEXT NOT NULL DEFAULT 'login',
    household_id INTEGER REFERENCES households(id) ON DELETE CASCADE,
    expires_at DATETIME NOT NULL,
    used_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_magic_links_token ON magic_links(token);
CREATE INDEX idx_magic_links_expires_at ON magic_links(expires_at);

-- Add household_id to existing tables (simple ALTER TABLE)
ALTER TABLE calendar_events ADD COLUMN household_id INTEGER DEFAULT 1 REFERENCES households(id) ON DELETE CASCADE;
ALTER TABLE chores ADD COLUMN household_id INTEGER DEFAULT 1 REFERENCES households(id) ON DELETE CASCADE;
ALTER TABLE grocery_lists ADD COLUMN household_id INTEGER DEFAULT 1 REFERENCES households(id) ON DELETE CASCADE;
ALTER TABLE notes ADD COLUMN household_id INTEGER DEFAULT 1 REFERENCES households(id) ON DELETE CASCADE;
ALTER TABLE rewards ADD COLUMN household_id INTEGER DEFAULT 1 REFERENCES households(id) ON DELETE CASCADE;

-- Rebuild family_members: UNIQUE(name) → UNIQUE(household_id, name)
CREATE TABLE family_members_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    color TEXT NOT NULL DEFAULT '#3B82F6',
    avatar_emoji TEXT NOT NULL DEFAULT '😀',
    pin TEXT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    household_id INTEGER DEFAULT 1 REFERENCES households(id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE(household_id, name)
);
INSERT INTO family_members_new (id, name, color, avatar_emoji, pin, sort_order, household_id, created_at, updated_at)
    SELECT id, name, color, avatar_emoji, pin, sort_order, 1, created_at, updated_at FROM family_members;
DROP TRIGGER IF EXISTS update_family_members_updated_at;
DROP TABLE family_members;
ALTER TABLE family_members_new RENAME TO family_members;

-- +goose StatementBegin
CREATE TRIGGER update_family_members_updated_at
AFTER UPDATE ON family_members
FOR EACH ROW
BEGIN
    UPDATE family_members SET updated_at = datetime('now') WHERE id = OLD.id;
END;
-- +goose StatementEnd

-- Rebuild chore_areas: UNIQUE(name) → UNIQUE(household_id, name)
CREATE TABLE chore_areas_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    household_id INTEGER DEFAULT 1 REFERENCES households(id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE(household_id, name)
);
INSERT INTO chore_areas_new (id, name, sort_order, household_id, created_at, updated_at)
    SELECT id, name, sort_order, 1, created_at, updated_at FROM chore_areas;
DROP TRIGGER IF EXISTS update_chore_areas_updated_at;
DROP TABLE chore_areas;
ALTER TABLE chore_areas_new RENAME TO chore_areas;

-- +goose StatementBegin
CREATE TRIGGER update_chore_areas_updated_at
AFTER UPDATE ON chore_areas
FOR EACH ROW
BEGIN
    UPDATE chore_areas SET updated_at = datetime('now') WHERE id = OLD.id;
END;
-- +goose StatementEnd

-- Rebuild grocery_categories: UNIQUE(name) → UNIQUE(household_id, name)
CREATE TABLE grocery_categories_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    household_id INTEGER DEFAULT 1 REFERENCES households(id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE(household_id, name)
);
INSERT INTO grocery_categories_new (id, name, sort_order, household_id, created_at)
    SELECT id, name, sort_order, 1, created_at FROM grocery_categories;
DROP TABLE grocery_categories;
ALTER TABLE grocery_categories_new RENAME TO grocery_categories;

-- Rebuild settings: key TEXT PRIMARY KEY → (id, household_id, key, value, UNIQUE(household_id, key))
CREATE TABLE settings_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    household_id INTEGER NOT NULL DEFAULT 1 REFERENCES households(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(household_id, key)
);
INSERT INTO settings_new (household_id, key, value, updated_at)
    SELECT 1, key, value, updated_at FROM settings;
DROP TABLE settings;
ALTER TABLE settings_new RENAME TO settings;

-- Indexes on household_id for all modified tables
CREATE INDEX idx_family_members_household_id ON family_members(household_id);
CREATE INDEX idx_chore_areas_household_id ON chore_areas(household_id);
CREATE INDEX idx_grocery_categories_household_id ON grocery_categories(household_id);
CREATE INDEX idx_settings_household_id ON settings(household_id);
CREATE INDEX idx_calendar_events_household_id ON calendar_events(household_id);
CREATE INDEX idx_chores_household_id ON chores(household_id);
CREATE INDEX idx_grocery_lists_household_id ON grocery_lists(household_id);
CREATE INDEX idx_notes_household_id ON notes(household_id);
CREATE INDEX idx_rewards_household_id ON rewards(household_id);

-- +goose Down
-- Reverse: rebuild settings back to key TEXT PRIMARY KEY
CREATE TABLE settings_old (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO settings_old (key, value, updated_at)
    SELECT key, value, updated_at FROM settings WHERE household_id = 1;
DROP TABLE settings;
ALTER TABLE settings_old RENAME TO settings;

-- Reverse: rebuild grocery_categories back to UNIQUE(name)
CREATE TABLE grocery_categories_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
INSERT INTO grocery_categories_old (id, name, sort_order, created_at)
    SELECT id, name, sort_order, created_at FROM grocery_categories WHERE household_id = 1;
DROP TABLE grocery_categories;
ALTER TABLE grocery_categories_old RENAME TO grocery_categories;

-- Reverse: rebuild chore_areas back to UNIQUE(name)
DROP TRIGGER IF EXISTS update_chore_areas_updated_at;
CREATE TABLE chore_areas_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
INSERT INTO chore_areas_old (id, name, sort_order, created_at, updated_at)
    SELECT id, name, sort_order, created_at, updated_at FROM chore_areas WHERE household_id = 1;
DROP TABLE chore_areas;
ALTER TABLE chore_areas_old RENAME TO chore_areas;

-- +goose StatementBegin
CREATE TRIGGER update_chore_areas_updated_at
AFTER UPDATE ON chore_areas
FOR EACH ROW
BEGIN
    UPDATE chore_areas SET updated_at = datetime('now') WHERE id = OLD.id;
END;
-- +goose StatementEnd

-- Reverse: rebuild family_members back to UNIQUE(name)
DROP TRIGGER IF EXISTS update_family_members_updated_at;
CREATE TABLE family_members_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    color TEXT NOT NULL DEFAULT '#3B82F6',
    avatar_emoji TEXT NOT NULL DEFAULT '😀',
    pin TEXT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
INSERT INTO family_members_old (id, name, color, avatar_emoji, pin, sort_order, created_at, updated_at)
    SELECT id, name, color, avatar_emoji, pin, sort_order, created_at, updated_at FROM family_members WHERE household_id = 1;
DROP TABLE family_members;
ALTER TABLE family_members_old RENAME TO family_members;

-- +goose StatementBegin
CREATE TRIGGER update_family_members_updated_at
AFTER UPDATE ON family_members
FOR EACH ROW
BEGIN
    UPDATE family_members SET updated_at = datetime('now') WHERE id = OLD.id;
END;
-- +goose StatementEnd

-- Remove household_id columns (can't drop columns in older SQLite, but ALTER TABLE DROP COLUMN works in 3.35+)
-- For safety, we just drop the new tables
DROP TABLE IF EXISTS magic_links;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS household_members;
DROP TRIGGER IF EXISTS update_household_members_updated_at;
DROP TRIGGER IF EXISTS update_users_updated_at;
DROP TABLE IF EXISTS users;
DROP TRIGGER IF EXISTS update_households_updated_at;
DROP TABLE IF EXISTS households;
