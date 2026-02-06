-- +goose Up
CREATE TABLE chore_areas (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- +goose StatementBegin
CREATE TRIGGER update_chore_areas_updated_at
AFTER UPDATE ON chore_areas
FOR EACH ROW
BEGIN
    UPDATE chore_areas SET updated_at = datetime('now') WHERE id = OLD.id;
END;
-- +goose StatementEnd

-- Seed default areas
INSERT INTO chore_areas (name, sort_order) VALUES ('Kitchen', 1);
INSERT INTO chore_areas (name, sort_order) VALUES ('Bathroom', 2);
INSERT INTO chore_areas (name, sort_order) VALUES ('Bedroom', 3);
INSERT INTO chore_areas (name, sort_order) VALUES ('Yard', 4);
INSERT INTO chore_areas (name, sort_order) VALUES ('General', 5);

CREATE TABLE chores (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    area_id INTEGER REFERENCES chore_areas(id) ON DELETE SET NULL,
    points INTEGER NOT NULL DEFAULT 0,
    recurrence_rule TEXT NOT NULL DEFAULT '',
    assigned_to INTEGER REFERENCES family_members(id) ON DELETE SET NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_chores_assigned_to ON chores(assigned_to);
CREATE INDEX idx_chores_area_id ON chores(area_id);

-- +goose StatementBegin
CREATE TRIGGER update_chores_updated_at
AFTER UPDATE ON chores
FOR EACH ROW
BEGIN
    UPDATE chores SET updated_at = datetime('now') WHERE id = OLD.id;
END;
-- +goose StatementEnd

CREATE TABLE chore_completions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chore_id INTEGER NOT NULL REFERENCES chores(id) ON DELETE CASCADE,
    completed_by INTEGER REFERENCES family_members(id) ON DELETE SET NULL,
    completed_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_chore_completions_chore_id ON chore_completions(chore_id);
CREATE INDEX idx_chore_completions_completed_by ON chore_completions(completed_by);
CREATE INDEX idx_chore_completions_completed_at ON chore_completions(completed_at);

-- +goose Down
DROP TABLE IF EXISTS chore_completions;
DROP TRIGGER IF EXISTS update_chores_updated_at;
DROP TABLE IF EXISTS chores;
DROP TRIGGER IF EXISTS update_chore_areas_updated_at;
DROP TABLE IF EXISTS chore_areas;
