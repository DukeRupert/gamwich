package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/dukerupert/gamwich/internal/model"
)

type ChoreStore struct {
	db *sql.DB
}

func NewChoreStore(db *sql.DB) *ChoreStore {
	return &ChoreStore{db: db}
}

// --- Area methods ---

func scanArea(scanner interface{ Scan(...any) error }) (*model.ChoreArea, error) {
	var a model.ChoreArea
	err := scanner.Scan(&a.ID, &a.Name, &a.SortOrder, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

const areaCols = `id, name, sort_order, created_at, updated_at`

func (s *ChoreStore) ListAreas() ([]model.ChoreArea, error) {
	rows, err := s.db.Query(`SELECT ` + areaCols + ` FROM chore_areas ORDER BY sort_order ASC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list areas: %w", err)
	}
	defer rows.Close()

	var areas []model.ChoreArea
	for rows.Next() {
		a, err := scanArea(rows)
		if err != nil {
			return nil, fmt.Errorf("scan area: %w", err)
		}
		areas = append(areas, *a)
	}
	return areas, rows.Err()
}

func (s *ChoreStore) GetAreaByID(id int64) (*model.ChoreArea, error) {
	row := s.db.QueryRow(`SELECT `+areaCols+` FROM chore_areas WHERE id = ?`, id)
	a, err := scanArea(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get area: %w", err)
	}
	return a, nil
}

func (s *ChoreStore) CreateArea(name string, sortOrder int) (*model.ChoreArea, error) {
	result, err := s.db.Exec(
		`INSERT INTO chore_areas (name, sort_order) VALUES (?, ?)`,
		name, sortOrder,
	)
	if err != nil {
		return nil, fmt.Errorf("insert area: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	return s.GetAreaByID(id)
}

func (s *ChoreStore) UpdateArea(id int64, name string, sortOrder int) (*model.ChoreArea, error) {
	_, err := s.db.Exec(
		`UPDATE chore_areas SET name = ?, sort_order = ? WHERE id = ?`,
		name, sortOrder, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update area: %w", err)
	}
	return s.GetAreaByID(id)
}

func (s *ChoreStore) DeleteArea(id int64) error {
	_, err := s.db.Exec(`DELETE FROM chore_areas WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete area: %w", err)
	}
	return nil
}

func (s *ChoreStore) UpdateAreaSortOrder(ids []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE chore_areas SET sort_order = ? WHERE id = ?`, i, id); err != nil {
			return fmt.Errorf("update sort order: %w", err)
		}
	}
	return tx.Commit()
}

// --- Chore methods ---

func scanChore(scanner interface{ Scan(...any) error }) (*model.Chore, error) {
	var c model.Chore
	var areaID sql.NullInt64
	var assignedTo sql.NullInt64

	err := scanner.Scan(
		&c.ID, &c.Title, &c.Description, &areaID, &c.Points,
		&c.RecurrenceRule, &assignedTo, &c.SortOrder,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if areaID.Valid {
		c.AreaID = &areaID.Int64
	}
	if assignedTo.Valid {
		c.AssignedTo = &assignedTo.Int64
	}
	return &c, nil
}

const choreCols = `id, title, description, area_id, points, recurrence_rule, assigned_to, sort_order, created_at, updated_at`

func (s *ChoreStore) Create(title, description string, areaID *int64, points int, recurrenceRule string, assignedTo *int64) (*model.Chore, error) {
	var aID sql.NullInt64
	if areaID != nil {
		aID = sql.NullInt64{Int64: *areaID, Valid: true}
	}
	var aTo sql.NullInt64
	if assignedTo != nil {
		aTo = sql.NullInt64{Int64: *assignedTo, Valid: true}
	}

	result, err := s.db.Exec(
		`INSERT INTO chores (title, description, area_id, points, recurrence_rule, assigned_to) VALUES (?, ?, ?, ?, ?, ?)`,
		title, description, aID, points, recurrenceRule, aTo,
	)
	if err != nil {
		return nil, fmt.Errorf("insert chore: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	return s.GetByID(id)
}

func (s *ChoreStore) GetByID(id int64) (*model.Chore, error) {
	row := s.db.QueryRow(`SELECT `+choreCols+` FROM chores WHERE id = ?`, id)
	c, err := scanChore(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get chore: %w", err)
	}
	return c, nil
}

func (s *ChoreStore) List() ([]model.Chore, error) {
	rows, err := s.db.Query(`SELECT ` + choreCols + ` FROM chores ORDER BY sort_order ASC, title ASC`)
	if err != nil {
		return nil, fmt.Errorf("list chores: %w", err)
	}
	defer rows.Close()

	var chores []model.Chore
	for rows.Next() {
		c, err := scanChore(rows)
		if err != nil {
			return nil, fmt.Errorf("scan chore: %w", err)
		}
		chores = append(chores, *c)
	}
	return chores, rows.Err()
}

func (s *ChoreStore) ListByAssignee(memberID int64) ([]model.Chore, error) {
	rows, err := s.db.Query(
		`SELECT `+choreCols+` FROM chores WHERE assigned_to = ? ORDER BY sort_order ASC, title ASC`,
		memberID,
	)
	if err != nil {
		return nil, fmt.Errorf("list chores by assignee: %w", err)
	}
	defer rows.Close()

	var chores []model.Chore
	for rows.Next() {
		c, err := scanChore(rows)
		if err != nil {
			return nil, fmt.Errorf("scan chore: %w", err)
		}
		chores = append(chores, *c)
	}
	return chores, rows.Err()
}

func (s *ChoreStore) ListByArea(areaID int64) ([]model.Chore, error) {
	rows, err := s.db.Query(
		`SELECT `+choreCols+` FROM chores WHERE area_id = ? ORDER BY sort_order ASC, title ASC`,
		areaID,
	)
	if err != nil {
		return nil, fmt.Errorf("list chores by area: %w", err)
	}
	defer rows.Close()

	var chores []model.Chore
	for rows.Next() {
		c, err := scanChore(rows)
		if err != nil {
			return nil, fmt.Errorf("scan chore: %w", err)
		}
		chores = append(chores, *c)
	}
	return chores, rows.Err()
}

func (s *ChoreStore) Update(id int64, title, description string, areaID *int64, points int, recurrenceRule string, assignedTo *int64) (*model.Chore, error) {
	var aID sql.NullInt64
	if areaID != nil {
		aID = sql.NullInt64{Int64: *areaID, Valid: true}
	}
	var aTo sql.NullInt64
	if assignedTo != nil {
		aTo = sql.NullInt64{Int64: *assignedTo, Valid: true}
	}

	_, err := s.db.Exec(
		`UPDATE chores SET title = ?, description = ?, area_id = ?, points = ?, recurrence_rule = ?, assigned_to = ? WHERE id = ?`,
		title, description, aID, points, recurrenceRule, aTo, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update chore: %w", err)
	}
	return s.GetByID(id)
}

func (s *ChoreStore) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM chores WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete chore: %w", err)
	}
	return nil
}

// --- Completion methods ---

func scanCompletion(scanner interface{ Scan(...any) error }) (*model.ChoreCompletion, error) {
	var c model.ChoreCompletion
	var completedBy sql.NullInt64

	err := scanner.Scan(&c.ID, &c.ChoreID, &completedBy, &c.CompletedAt)
	if err != nil {
		return nil, err
	}

	if completedBy.Valid {
		c.CompletedBy = &completedBy.Int64
	}
	return &c, nil
}

const completionCols = `id, chore_id, completed_by, completed_at`

func (s *ChoreStore) CreateCompletion(choreID int64, completedBy *int64) (*model.ChoreCompletion, error) {
	var cBy sql.NullInt64
	if completedBy != nil {
		cBy = sql.NullInt64{Int64: *completedBy, Valid: true}
	}

	result, err := s.db.Exec(
		`INSERT INTO chore_completions (chore_id, completed_by) VALUES (?, ?)`,
		choreID, cBy,
	)
	if err != nil {
		return nil, fmt.Errorf("insert completion: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	row := s.db.QueryRow(`SELECT `+completionCols+` FROM chore_completions WHERE id = ?`, id)
	return scanCompletion(row)
}

func (s *ChoreStore) DeleteCompletion(id int64) error {
	_, err := s.db.Exec(`DELETE FROM chore_completions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete completion: %w", err)
	}
	return nil
}

func (s *ChoreStore) ListCompletionsByChore(choreID int64) ([]model.ChoreCompletion, error) {
	rows, err := s.db.Query(
		`SELECT `+completionCols+` FROM chore_completions WHERE chore_id = ? ORDER BY completed_at DESC`,
		choreID,
	)
	if err != nil {
		return nil, fmt.Errorf("list completions: %w", err)
	}
	defer rows.Close()

	var completions []model.ChoreCompletion
	for rows.Next() {
		c, err := scanCompletion(rows)
		if err != nil {
			return nil, fmt.Errorf("scan completion: %w", err)
		}
		completions = append(completions, *c)
	}
	return completions, rows.Err()
}

func (s *ChoreStore) ListCompletionsByDateRange(start, end time.Time) ([]model.ChoreCompletion, error) {
	rows, err := s.db.Query(
		`SELECT `+completionCols+` FROM chore_completions WHERE completed_at >= ? AND completed_at < ? ORDER BY completed_at DESC`,
		start.UTC(), end.UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("list completions by range: %w", err)
	}
	defer rows.Close()

	var completions []model.ChoreCompletion
	for rows.Next() {
		c, err := scanCompletion(rows)
		if err != nil {
			return nil, fmt.Errorf("scan completion: %w", err)
		}
		completions = append(completions, *c)
	}
	return completions, rows.Err()
}

func (s *ChoreStore) LastCompletionForChore(choreID int64) (*model.ChoreCompletion, error) {
	row := s.db.QueryRow(
		`SELECT `+completionCols+` FROM chore_completions WHERE chore_id = ? ORDER BY completed_at DESC LIMIT 1`,
		choreID,
	)
	c, err := scanCompletion(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("last completion: %w", err)
	}
	return c, nil
}
