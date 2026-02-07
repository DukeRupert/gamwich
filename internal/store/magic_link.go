package store

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
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
		&ml.ExpiresAt, &usedAt, &ml.Attempts, &ml.CreatedAt,
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

const magicLinkCols = `id, token, email, purpose, household_id, expires_at, used_at, attempts, created_at`

// generateCode returns a 6-digit numeric code (100000–999999).
func generateCode() (string, error) {
	// Range: 100000 to 999999 (900000 values)
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}
	code := n.Int64() + 100000
	return fmt.Sprintf("%06d", code), nil
}

// Create generates a new magic link with a 6-digit numeric code and 15-minute expiry.
// Any previous pending codes for the same email are invalidated first.
func (s *MagicLinkStore) Create(email, purpose string, householdID *int64) (*model.MagicLink, error) {
	// Invalidate any previous pending codes for this email
	_, err := s.db.Exec(
		`UPDATE magic_links SET used_at = datetime('now') WHERE email = ? AND used_at IS NULL AND expires_at > datetime('now')`,
		email,
	)
	if err != nil {
		return nil, fmt.Errorf("invalidate previous codes: %w", err)
	}

	code, err := generateCode()
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().UTC().Add(15 * time.Minute)

	var hID sql.NullInt64
	if householdID != nil {
		hID = sql.NullInt64{Int64: *householdID, Valid: true}
	}

	result, err := s.db.Exec(
		`INSERT INTO magic_links (token, email, purpose, household_id, expires_at) VALUES (?, ?, ?, ?, ?)`,
		code, email, purpose, hID, expiresAt,
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

// GetByEmailAndCode returns the magic link matching the email and code, or nil if not found/expired/used.
func (s *MagicLinkStore) GetByEmailAndCode(email, code string) (*model.MagicLink, error) {
	row := s.db.QueryRow(
		`SELECT `+magicLinkCols+` FROM magic_links WHERE email = ? AND token = ? AND expires_at > datetime('now') AND used_at IS NULL`,
		email, code,
	)
	ml, err := scanMagicLink(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get magic link by email and code: %w", err)
	}
	return ml, nil
}

// GetLatestByEmail returns the most recent valid (unexpired, unused) code for an email.
func (s *MagicLinkStore) GetLatestByEmail(email string) (*model.MagicLink, error) {
	row := s.db.QueryRow(
		`SELECT `+magicLinkCols+` FROM magic_links WHERE email = ? AND expires_at > datetime('now') AND used_at IS NULL ORDER BY created_at DESC LIMIT 1`,
		email,
	)
	ml, err := scanMagicLink(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest magic link by email: %w", err)
	}
	return ml, nil
}

// IncrementAttempts increments the attempt count and returns the new value.
func (s *MagicLinkStore) IncrementAttempts(id int64) (int, error) {
	_, err := s.db.Exec(
		`UPDATE magic_links SET attempts = attempts + 1 WHERE id = ?`,
		id,
	)
	if err != nil {
		return 0, fmt.Errorf("increment attempts: %w", err)
	}

	var attempts int
	err = s.db.QueryRow(`SELECT attempts FROM magic_links WHERE id = ?`, id).Scan(&attempts)
	if err != nil {
		return 0, fmt.Errorf("read attempts: %w", err)
	}
	return attempts, nil
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
