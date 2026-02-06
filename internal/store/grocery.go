package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/dukerupert/gamwich/internal/model"
)

type GroceryStore struct {
	db *sql.DB
}

func NewGroceryStore(db *sql.DB) *GroceryStore {
	return &GroceryStore{db: db}
}

// --- Category methods ---

func scanCategory(scanner interface{ Scan(...any) error }) (*model.GroceryCategory, error) {
	var c model.GroceryCategory
	err := scanner.Scan(&c.ID, &c.Name, &c.SortOrder, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

const categoryCols = `id, name, sort_order, created_at`

func (s *GroceryStore) ListCategories() ([]model.GroceryCategory, error) {
	rows, err := s.db.Query(`SELECT ` + categoryCols + ` FROM grocery_categories ORDER BY sort_order ASC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	var categories []model.GroceryCategory
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		categories = append(categories, *c)
	}
	return categories, rows.Err()
}

// --- List methods ---

func scanList(scanner interface{ Scan(...any) error }) (*model.GroceryList, error) {
	var l model.GroceryList
	err := scanner.Scan(&l.ID, &l.Name, &l.SortOrder, &l.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

const listCols = `id, name, sort_order, created_at`

func (s *GroceryStore) GetDefaultList() (*model.GroceryList, error) {
	row := s.db.QueryRow(`SELECT ` + listCols + ` FROM grocery_lists ORDER BY sort_order ASC LIMIT 1`)
	l, err := scanList(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get default list: %w", err)
	}
	return l, nil
}

// --- Item methods ---

func scanItem(scanner interface{ Scan(...any) error }) (*model.GroceryItem, error) {
	var item model.GroceryItem
	var checkedBy, addedBy sql.NullInt64
	var checkedAt sql.NullTime
	var checked int

	err := scanner.Scan(
		&item.ID, &item.ListID, &item.Name, &item.Quantity, &item.Unit,
		&item.Notes, &item.Category, &checked, &checkedBy, &checkedAt,
		&addedBy, &item.SortOrder, &item.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	item.Checked = checked != 0
	if checkedBy.Valid {
		item.CheckedBy = &checkedBy.Int64
	}
	if checkedAt.Valid {
		item.CheckedAt = &checkedAt.Time
	}
	if addedBy.Valid {
		item.AddedBy = &addedBy.Int64
	}
	return &item, nil
}

const itemCols = `id, list_id, name, quantity, unit, notes, category, checked, checked_by, checked_at, added_by, sort_order, created_at`

func (s *GroceryStore) GetItemByID(id int64) (*model.GroceryItem, error) {
	row := s.db.QueryRow(`SELECT `+itemCols+` FROM grocery_items WHERE id = ?`, id)
	item, err := scanItem(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get item: %w", err)
	}
	return item, nil
}

func (s *GroceryStore) CreateItem(listID int64, name, quantity, unit, notes, category string, addedBy *int64) (*model.GroceryItem, error) {
	var aBy sql.NullInt64
	if addedBy != nil {
		aBy = sql.NullInt64{Int64: *addedBy, Valid: true}
	}

	result, err := s.db.Exec(
		`INSERT INTO grocery_items (list_id, name, quantity, unit, notes, category, added_by) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		listID, name, quantity, unit, notes, category, aBy,
	)
	if err != nil {
		return nil, fmt.Errorf("insert item: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	return s.GetItemByID(id)
}

func (s *GroceryStore) ListItemsByList(listID int64) ([]model.GroceryItem, error) {
	rows, err := s.db.Query(
		`SELECT `+itemCols+` FROM grocery_items WHERE list_id = ? ORDER BY checked ASC, category ASC, sort_order ASC, created_at ASC`,
		listID,
	)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	defer rows.Close()

	var items []model.GroceryItem
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (s *GroceryStore) UpdateItem(id int64, name, quantity, unit, notes, category string) (*model.GroceryItem, error) {
	_, err := s.db.Exec(
		`UPDATE grocery_items SET name = ?, quantity = ?, unit = ?, notes = ?, category = ? WHERE id = ?`,
		name, quantity, unit, notes, category, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update item: %w", err)
	}
	return s.GetItemByID(id)
}

func (s *GroceryStore) DeleteItem(id int64) error {
	_, err := s.db.Exec(`DELETE FROM grocery_items WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete item: %w", err)
	}
	return nil
}

func (s *GroceryStore) ToggleChecked(id int64, checkedBy *int64) (*model.GroceryItem, error) {
	item, err := s.GetItemByID(id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}

	if item.Checked {
		// Uncheck
		_, err = s.db.Exec(
			`UPDATE grocery_items SET checked = 0, checked_by = NULL, checked_at = NULL WHERE id = ?`,
			id,
		)
	} else {
		// Check
		var cBy sql.NullInt64
		if checkedBy != nil {
			cBy = sql.NullInt64{Int64: *checkedBy, Valid: true}
		}
		_, err = s.db.Exec(
			`UPDATE grocery_items SET checked = 1, checked_by = ?, checked_at = ? WHERE id = ?`,
			cBy, time.Now().UTC(), id,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("toggle checked: %w", err)
	}
	return s.GetItemByID(id)
}

func (s *GroceryStore) ClearChecked(listID int64) (int64, error) {
	result, err := s.db.Exec(
		`DELETE FROM grocery_items WHERE list_id = ? AND checked = 1`,
		listID,
	)
	if err != nil {
		return 0, fmt.Errorf("clear checked: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return count, nil
}

func (s *GroceryStore) CountUnchecked(listID int64) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM grocery_items WHERE list_id = ? AND checked = 0`,
		listID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count unchecked: %w", err)
	}
	return count, nil
}
