package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/dukerupert/gamwich/internal/billing/model"
)

type SessionStore struct {
	db *sql.DB
}

func NewSessionStore(db *sql.DB) *SessionStore {
	return &SessionStore{db: db}
}

func scanSession(scanner interface{ Scan(...any) error }) (*model.Session, error) {
	var s model.Session
	err := scanner.Scan(&s.ID, &s.Token, &s.AccountID, &s.ExpiresAt, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

const sessionCols = `id, token, account_id, expires_at, created_at`

// Create generates a new session with a crypto-random token and 90-day expiry.
func (s *SessionStore) Create(accountID int64) (*model.Session, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	expiresAt := time.Now().UTC().Add(90 * 24 * time.Hour)

	result, err := s.db.Exec(
		`INSERT INTO sessions (token, account_id, expires_at) VALUES (?, ?, ?)`,
		token, accountID, expiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	row := s.db.QueryRow(`SELECT `+sessionCols+` FROM sessions WHERE id = ?`, id)
	return scanSession(row)
}

// GetByToken returns the session for the given token, or nil if expired or not found.
func (s *SessionStore) GetByToken(token string) (*model.Session, error) {
	row := s.db.QueryRow(
		`SELECT `+sessionCols+` FROM sessions WHERE token = ? AND expires_at > datetime('now')`,
		token,
	)
	sess, err := scanSession(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session by token: %w", err)
	}
	return sess, nil
}

func (s *SessionStore) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *SessionStore) DeleteExpired() (int64, error) {
	result, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at <= datetime('now')`)
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return count, nil
}

func (s *SessionStore) DeleteByAccountID(accountID int64) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE account_id = ?`, accountID)
	if err != nil {
		return fmt.Errorf("delete sessions by account: %w", err)
	}
	return nil
}
