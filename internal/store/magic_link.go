package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/dukerupert/gamwich/internal/model"
)

type MagicLinkStore struct {
	db *sql.DB
}

func NewMagicLinkStore(db *sql.DB) *MagicLinkStore {
	return &MagicLinkStore{db: db}
}

func scanMagicLink(scanner interface{ Scan(...any) error }) (*model.MagicLink, error) {
	var ml model.MagicLink
	var householdID sql.NullInt64
	var usedAt sql.NullTime

	err := scanner.Scan(
		&ml.ID, &ml.Token, &ml.Email, &ml.Purpose, &householdID,
		&ml.ExpiresAt, &usedAt, &ml.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if householdID.Valid {
		ml.HouseholdID = &householdID.Int64
	}
	if usedAt.Valid {
		ml.UsedAt = &usedAt.Time
	}
	return &ml, nil
}

const magicLinkCols = `id, token, email, purpose, household_id, expires_at, used_at, created_at`

// Create generates a new magic link with a crypto-random token and 15-minute expiry.
func (s *MagicLinkStore) Create(email, purpose string, householdID *int64) (*model.MagicLink, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	expiresAt := time.Now().UTC().Add(15 * time.Minute)

	var hID sql.NullInt64
	if householdID != nil {
		hID = sql.NullInt64{Int64: *householdID, Valid: true}
	}

	result, err := s.db.Exec(
		`INSERT INTO magic_links (token, email, purpose, household_id, expires_at) VALUES (?, ?, ?, ?, ?)`,
		token, email, purpose, hID, expiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert magic link: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	row := s.db.QueryRow(`SELECT `+magicLinkCols+` FROM magic_links WHERE id = ?`, id)
	return scanMagicLink(row)
}

// GetByToken returns the magic link for the given token, or nil if expired or used.
func (s *MagicLinkStore) GetByToken(token string) (*model.MagicLink, error) {
	row := s.db.QueryRow(
		`SELECT `+magicLinkCols+` FROM magic_links WHERE token = ? AND expires_at > datetime('now') AND used_at IS NULL`,
		token,
	)
	ml, err := scanMagicLink(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get magic link by token: %w", err)
	}
	return ml, nil
}

func (s *MagicLinkStore) MarkUsed(id int64) error {
	_, err := s.db.Exec(
		`UPDATE magic_links SET used_at = datetime('now') WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("mark magic link used: %w", err)
	}
	return nil
}

func (s *MagicLinkStore) DeleteExpired() (int64, error) {
	result, err := s.db.Exec(`DELETE FROM magic_links WHERE expires_at <= datetime('now')`)
	if err != nil {
		return 0, fmt.Errorf("delete expired magic links: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return count, nil
}
