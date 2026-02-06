package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/dukerupert/gamwich/internal/model"
)

type NoteStore struct {
	db *sql.DB
}

func NewNoteStore(db *sql.DB) *NoteStore {
	return &NoteStore{db: db}
}

func scanNote(scanner interface{ Scan(...any) error }) (*model.Note, error) {
	var n model.Note
	var authorID sql.NullInt64
	var expiresAt sql.NullTime
	var pinned int

	err := scanner.Scan(
		&n.ID, &n.Title, &n.Body, &authorID, &pinned,
		&n.Priority, &expiresAt, &n.CreatedAt, &n.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	n.Pinned = pinned != 0
	if authorID.Valid {
		n.AuthorID = &authorID.Int64
	}
	if expiresAt.Valid {
		n.ExpiresAt = &expiresAt.Time
	}
	return &n, nil
}

const noteCols = `id, title, body, author_id, pinned, priority, expires_at, created_at, updated_at`

func (s *NoteStore) Create(title, body string, authorID *int64, pinned bool, priority string, expiresAt *time.Time) (*model.Note, error) {
	var aID sql.NullInt64
	if authorID != nil {
		aID = sql.NullInt64{Int64: *authorID, Valid: true}
	}
	var exp sql.NullTime
	if expiresAt != nil {
		exp = sql.NullTime{Time: *expiresAt, Valid: true}
	}
	var p int
	if pinned {
		p = 1
	}

	result, err := s.db.Exec(
		`INSERT INTO notes (title, body, author_id, pinned, priority, expires_at) VALUES (?, ?, ?, ?, ?, ?)`,
		title, body, aID, p, priority, exp,
	)
	if err != nil {
		return nil, fmt.Errorf("insert note: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	return s.GetByID(id)
}

func (s *NoteStore) GetByID(id int64) (*model.Note, error) {
	row := s.db.QueryRow(`SELECT `+noteCols+` FROM notes WHERE id = ?`, id)
	n, err := scanNote(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get note: %w", err)
	}
	return n, nil
}

// List returns non-expired notes ordered by pinned DESC, priority (urgent > normal > fun), created_at DESC.
// It lazily cleans up expired notes before querying.
func (s *NoteStore) List() ([]model.Note, error) {
	s.DeleteExpired()

	rows, err := s.db.Query(
		`SELECT ` + noteCols + ` FROM notes
		 WHERE expires_at IS NULL OR expires_at > datetime('now')
		 ORDER BY pinned DESC,
		   CASE priority WHEN 'urgent' THEN 0 WHEN 'normal' THEN 1 WHEN 'fun' THEN 2 ELSE 3 END,
		   created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()

	var notes []model.Note
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		notes = append(notes, *n)
	}
	return notes, rows.Err()
}

// ListPinned returns pinned, non-expired notes for the dashboard widget.
func (s *NoteStore) ListPinned() ([]model.Note, error) {
	rows, err := s.db.Query(
		`SELECT `+noteCols+` FROM notes
		 WHERE pinned = 1 AND (expires_at IS NULL OR expires_at > datetime('now'))
		 ORDER BY CASE priority WHEN 'urgent' THEN 0 WHEN 'normal' THEN 1 WHEN 'fun' THEN 2 ELSE 3 END,
		   created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list pinned notes: %w", err)
	}
	defer rows.Close()

	var notes []model.Note
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		notes = append(notes, *n)
	}
	return notes, rows.Err()
}

func (s *NoteStore) Update(id int64, title, body string, authorID *int64, pinned bool, priority string, expiresAt *time.Time) (*model.Note, error) {
	var aID sql.NullInt64
	if authorID != nil {
		aID = sql.NullInt64{Int64: *authorID, Valid: true}
	}
	var exp sql.NullTime
	if expiresAt != nil {
		exp = sql.NullTime{Time: *expiresAt, Valid: true}
	}
	var p int
	if pinned {
		p = 1
	}

	_, err := s.db.Exec(
		`UPDATE notes SET title = ?, body = ?, author_id = ?, pinned = ?, priority = ?, expires_at = ? WHERE id = ?`,
		title, body, aID, p, priority, exp, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update note: %w", err)
	}
	return s.GetByID(id)
}

func (s *NoteStore) TogglePinned(id int64) (*model.Note, error) {
	note, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	if note == nil {
		return nil, nil
	}

	newPinned := 0
	if !note.Pinned {
		newPinned = 1
	}

	_, err = s.db.Exec(`UPDATE notes SET pinned = ? WHERE id = ?`, newPinned, id)
	if err != nil {
		return nil, fmt.Errorf("toggle pinned: %w", err)
	}
	return s.GetByID(id)
}

func (s *NoteStore) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM notes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	return nil
}

// DeleteExpired removes expired notes and returns the number deleted.
func (s *NoteStore) DeleteExpired() (int64, error) {
	result, err := s.db.Exec(`DELETE FROM notes WHERE expires_at IS NOT NULL AND expires_at <= datetime('now')`)
	if err != nil {
		return 0, fmt.Errorf("delete expired: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return count, nil
}
