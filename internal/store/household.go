package store

import (
	"database/sql"
	"fmt"

	"github.com/dukerupert/gamwich/internal/model"
)

type HouseholdStore struct {
	db *sql.DB
}

func NewHouseholdStore(db *sql.DB) *HouseholdStore {
	return &HouseholdStore{db: db}
}

func scanHousehold(scanner interface{ Scan(...any) error }) (*model.Household, error) {
	var h model.Household
	err := scanner.Scan(&h.ID, &h.Name, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func scanHouseholdMember(scanner interface{ Scan(...any) error }) (*model.HouseholdMember, error) {
	var m model.HouseholdMember
	err := scanner.Scan(&m.ID, &m.HouseholdID, &m.UserID, &m.Role, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

const householdCols = `id, name, created_at, updated_at`
const householdMemberCols = `id, household_id, user_id, role, created_at, updated_at`

func (s *HouseholdStore) Create(name string) (*model.Household, error) {
	result, err := s.db.Exec(`INSERT INTO households (name) VALUES (?)`, name)
	if err != nil {
		return nil, fmt.Errorf("insert household: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	return s.GetByID(id)
}

func (s *HouseholdStore) GetByID(id int64) (*model.Household, error) {
	row := s.db.QueryRow(`SELECT `+householdCols+` FROM households WHERE id = ?`, id)
	h, err := scanHousehold(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get household: %w", err)
	}
	return h, nil
}

func (s *HouseholdStore) Update(id int64, name string) (*model.Household, error) {
	_, err := s.db.Exec(`UPDATE households SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		return nil, fmt.Errorf("update household: %w", err)
	}
	return s.GetByID(id)
}

func (s *HouseholdStore) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM households WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete household: %w", err)
	}
	return nil
}

func (s *HouseholdStore) AddMember(householdID, userID int64, role string) (*model.HouseholdMember, error) {
	result, err := s.db.Exec(
		`INSERT INTO household_members (household_id, user_id, role) VALUES (?, ?, ?)`,
		householdID, userID, role,
	)
	if err != nil {
		return nil, fmt.Errorf("add member: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	row := s.db.QueryRow(`SELECT `+householdMemberCols+` FROM household_members WHERE id = ?`, id)
	return scanHouseholdMember(row)
}

func (s *HouseholdStore) RemoveMember(householdID, userID int64) error {
	_, err := s.db.Exec(
		`DELETE FROM household_members WHERE household_id = ? AND user_id = ?`,
		householdID, userID,
	)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	return nil
}

func (s *HouseholdStore) GetMember(householdID, userID int64) (*model.HouseholdMember, error) {
	row := s.db.QueryRow(
		`SELECT `+householdMemberCols+` FROM household_members WHERE household_id = ? AND user_id = ?`,
		householdID, userID,
	)
	m, err := scanHouseholdMember(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get member: %w", err)
	}
	return m, nil
}

func (s *HouseholdStore) ListMembers(householdID int64) ([]model.HouseholdMember, error) {
	rows, err := s.db.Query(
		`SELECT `+householdMemberCols+` FROM household_members WHERE household_id = ? ORDER BY created_at ASC`,
		householdID,
	)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	var members []model.HouseholdMember
	for rows.Next() {
		m, err := scanHouseholdMember(rows)
		if err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, *m)
	}
	return members, rows.Err()
}

func (s *HouseholdStore) ListHouseholdsForUser(userID int64) ([]model.Household, error) {
	rows, err := s.db.Query(
		`SELECT h.id, h.name, h.created_at, h.updated_at
		 FROM households h
		 JOIN household_members hm ON h.id = hm.household_id
		 WHERE hm.user_id = ?
		 ORDER BY h.name ASC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list households for user: %w", err)
	}
	defer rows.Close()

	var households []model.Household
	for rows.Next() {
		h, err := scanHousehold(rows)
		if err != nil {
			return nil, fmt.Errorf("scan household: %w", err)
		}
		households = append(households, *h)
	}
	return households, rows.Err()
}

func (s *HouseholdStore) UpdateMemberRole(householdID, userID int64, role string) (*model.HouseholdMember, error) {
	_, err := s.db.Exec(
		`UPDATE household_members SET role = ? WHERE household_id = ? AND user_id = ?`,
		role, householdID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("update member role: %w", err)
	}
	return s.GetMember(householdID, userID)
}

// SeedDefaults inserts default chore areas, grocery categories, a grocery list,
// and settings for a new household in a single transaction.
func (s *HouseholdStore) SeedDefaults(householdID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Chore areas
	areas := []struct {
		name      string
		sortOrder int
	}{
		{"Kitchen", 1}, {"Bathroom", 2}, {"Bedroom", 3}, {"Yard", 4}, {"General", 5},
	}
	for _, a := range areas {
		if _, err := tx.Exec(
			`INSERT INTO chore_areas (name, sort_order, household_id) VALUES (?, ?, ?)`,
			a.name, a.sortOrder, householdID,
		); err != nil {
			return fmt.Errorf("seed chore area %q: %w", a.name, err)
		}
	}

	// Grocery categories
	categories := []struct {
		name      string
		sortOrder int
	}{
		{"Produce", 1}, {"Dairy", 2}, {"Meat & Seafood", 3}, {"Bakery", 4},
		{"Pantry", 5}, {"Frozen", 6}, {"Beverages", 7}, {"Snacks", 8},
		{"Household", 9}, {"Personal Care", 10}, {"Other", 11},
	}
	for _, c := range categories {
		if _, err := tx.Exec(
			`INSERT INTO grocery_categories (name, sort_order, household_id) VALUES (?, ?, ?)`,
			c.name, c.sortOrder, householdID,
		); err != nil {
			return fmt.Errorf("seed grocery category %q: %w", c.name, err)
		}
	}

	// Default grocery list
	if _, err := tx.Exec(
		`INSERT INTO grocery_lists (name, sort_order, household_id) VALUES ('Grocery', 0, ?)`,
		householdID,
	); err != nil {
		return fmt.Errorf("seed grocery list: %w", err)
	}

	// Default settings
	settings := []struct {
		key   string
		value string
	}{
		{"idle_timeout_minutes", "5"},
		{"quiet_hours_enabled", "false"},
		{"quiet_hours_start", "22:00"},
		{"quiet_hours_end", "06:00"},
		{"burn_in_prevention", "true"},
		{"weather_latitude", ""},
		{"weather_longitude", ""},
		{"weather_units", "fahrenheit"},
		{"theme_mode", "manual"},
		{"theme_selected", "garden"},
		{"theme_light", "garden"},
		{"theme_dark", "forest"},
		{"rewards_leaderboard_enabled", "true"},
	}
	for _, s := range settings {
		if _, err := tx.Exec(
			`INSERT INTO settings (household_id, key, value) VALUES (?, ?, ?)`,
			householdID, s.key, s.value,
		); err != nil {
			return fmt.Errorf("seed setting %q: %w", s.key, err)
		}
	}

	return tx.Commit()
}
