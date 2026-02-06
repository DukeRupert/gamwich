-- +goose Up
CREATE TABLE grocery_categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Seed default categories
INSERT INTO grocery_categories (name, sort_order) VALUES ('Produce', 1);
INSERT INTO grocery_categories (name, sort_order) VALUES ('Dairy', 2);
INSERT INTO grocery_categories (name, sort_order) VALUES ('Meat & Seafood', 3);
INSERT INTO grocery_categories (name, sort_order) VALUES ('Bakery', 4);
INSERT INTO grocery_categories (name, sort_order) VALUES ('Pantry', 5);
INSERT INTO grocery_categories (name, sort_order) VALUES ('Frozen', 6);
INSERT INTO grocery_categories (name, sort_order) VALUES ('Beverages', 7);
INSERT INTO grocery_categories (name, sort_order) VALUES ('Snacks', 8);
INSERT INTO grocery_categories (name, sort_order) VALUES ('Household', 9);
INSERT INTO grocery_categories (name, sort_order) VALUES ('Personal Care', 10);
INSERT INTO grocery_categories (name, sort_order) VALUES ('Other', 11);

CREATE TABLE grocery_lists (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Seed default list
INSERT INTO grocery_lists (name, sort_order) VALUES ('Grocery', 0);

CREATE TABLE grocery_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    list_id INTEGER NOT NULL REFERENCES grocery_lists(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    quantity TEXT NOT NULL DEFAULT '',
    unit TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT 'Other',
    checked INTEGER NOT NULL DEFAULT 0,
    checked_by INTEGER REFERENCES family_members(id) ON DELETE SET NULL,
    checked_at DATETIME,
    added_by INTEGER REFERENCES family_members(id) ON DELETE SET NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_grocery_items_list_id ON grocery_items(list_id);
CREATE INDEX idx_grocery_items_checked ON grocery_items(checked);
CREATE INDEX idx_grocery_items_added_by ON grocery_items(added_by);
CREATE INDEX idx_grocery_items_checked_by ON grocery_items(checked_by);

-- +goose Down
DROP TABLE IF EXISTS grocery_items;
DROP TABLE IF EXISTS grocery_lists;
DROP TABLE IF EXISTS grocery_categories;
