package store

import (
	"database/sql"
	"fmt"

	"github.com/dukerupert/gamwich/internal/model"
)

type FamilyMemberStore struct {
	db *sql.DB
}

func NewFamilyMemberStore(db *sql.DB) *FamilyMemberStore {
	return &FamilyMemberStore{db: db}
}

func (s *FamilyMemberStore) Create(name, color, avatarEmoji string) (*model.FamilyMember, error) {
	var maxOrder int
	err := s.db.QueryRow("SELECT COALESCE(MAX(sort_order), -1) FROM family_members").Scan(&maxOrder)
	if err != nil {
		return nil, fmt.Errorf("query max sort_order: %w", err)
	}

	result, err := s.db.Exec(
		"INSERT INTO family_members (name, color, avatar_emoji, sort_order) VALUES (?, ?, ?, ?)",
		name, color, avatarEmoji, maxOrder+1,
	)
	if err != nil {
		return nil, fmt.Errorf("insert family member: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	return s.GetByID(id)
}

func (s *FamilyMemberStore) List() ([]model.FamilyMember, error) {
	rows, err := s.db.Query(
		"SELECT id, name, color, avatar_emoji, pin IS NOT NULL, sort_order, created_at, updated_at FROM family_members ORDER BY sort_order",
	)
	if err != nil {
		return nil, fmt.Errorf("query family members: %w", err)
	}
	defer rows.Close()

	var members []model.FamilyMember
	for rows.Next() {
		var m model.FamilyMember
		if err := rows.Scan(&m.ID, &m.Name, &m.Color, &m.AvatarEmoji, &m.HasPIN, &m.SortOrder, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan family member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (s *FamilyMemberStore) GetByID(id int64) (*model.FamilyMember, error) {
	var m model.FamilyMember
	err := s.db.QueryRow(
		"SELECT id, name, color, avatar_emoji, pin IS NOT NULL, sort_order, created_at, updated_at FROM family_members WHERE id = ?",
		id,
	).Scan(&m.ID, &m.Name, &m.Color, &m.AvatarEmoji, &m.HasPIN, &m.SortOrder, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query family member: %w", err)
	}
	return &m, nil
}

func (s *FamilyMemberStore) Update(id int64, name, color, avatarEmoji string) (*model.FamilyMember, error) {
	_, err := s.db.Exec(
		"UPDATE family_members SET name = ?, color = ?, avatar_emoji = ? WHERE id = ?",
		name, color, avatarEmoji, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update family member: %w", err)
	}
	return s.GetByID(id)
}

func (s *FamilyMemberStore) Delete(id int64) error {
	_, err := s.db.Exec("DELETE FROM family_members WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete family member: %w", err)
	}
	return nil
}

func (s *FamilyMemberStore) UpdateSortOrder(ids []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("UPDATE family_members SET sort_order = ? WHERE id = ?")
	if err != nil {
		return fmt.Errorf("prepare stmt: %w", err)
	}
	defer stmt.Close()

	for i, id := range ids {
		if _, err := stmt.Exec(i, id); err != nil {
			return fmt.Errorf("update sort order for id %d: %w", id, err)
		}
	}

	return tx.Commit()
}

func (s *FamilyMemberStore) SetPIN(id int64, hashedPIN string) error {
	_, err := s.db.Exec("UPDATE family_members SET pin = ? WHERE id = ?", hashedPIN, id)
	if err != nil {
		return fmt.Errorf("set pin: %w", err)
	}
	return nil
}

func (s *FamilyMemberStore) ClearPIN(id int64) error {
	_, err := s.db.Exec("UPDATE family_members SET pin = NULL WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("clear pin: %w", err)
	}
	return nil
}

func (s *FamilyMemberStore) GetPINHash(id int64) (string, error) {
	var pin sql.NullString
	err := s.db.QueryRow("SELECT pin FROM family_members WHERE id = ?", id).Scan(&pin)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("family member not found")
	}
	if err != nil {
		return "", fmt.Errorf("query pin: %w", err)
	}
	if !pin.Valid {
		return "", nil
	}
	return pin.String, nil
}

func (s *FamilyMemberStore) NameExists(name string, excludeID int64) (bool, error) {
	var count int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM family_members WHERE name = ? AND id != ?",
		name, excludeID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check name exists: %w", err)
	}
	return count > 0, nil
}
