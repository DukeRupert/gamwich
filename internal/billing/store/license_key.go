package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/dukerupert/gamwich/internal/billing/model"
)

type LicenseKeyStore struct {
	db *sql.DB
}

func NewLicenseKeyStore(db *sql.DB) *LicenseKeyStore {
	return &LicenseKeyStore{db: db}
}

func scanLicenseKey(scanner interface{ Scan(...any) error }) (*model.LicenseKey, error) {
	var lk model.LicenseKey
	var activatedAt sql.NullTime
	var expiresAt sql.NullTime
	err := scanner.Scan(
		&lk.ID, &lk.AccountID, &lk.SubscriptionID, &lk.Key, &lk.Plan,
		&lk.Features, &activatedAt, &expiresAt, &lk.CreatedAt, &lk.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if activatedAt.Valid {
		lk.ActivatedAt = &activatedAt.Time
	}
	if expiresAt.Valid {
		lk.ExpiresAt = &expiresAt.Time
	}
	return &lk, nil
}

const licenseKeyCols = `id, account_id, subscription_id, key, plan, features, activated_at, expires_at, created_at, updated_at`

// generateKey creates a license key in the format GW-XXXX-XXXX-XXXX-XXXX.
func generateKey() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}
	h := strings.ToUpper(hex.EncodeToString(b))
	return fmt.Sprintf("GW-%s-%s-%s-%s", h[0:4], h[4:8], h[8:12], h[12:16]), nil
}

func (s *LicenseKeyStore) Create(accountID, subscriptionID int64, plan, features string) (*model.LicenseKey, error) {
	key, err := generateKey()
	if err != nil {
		return nil, err
	}

	result, err := s.db.Exec(
		`INSERT INTO license_keys (account_id, subscription_id, key, plan, features) VALUES (?, ?, ?, ?, ?)`,
		accountID, subscriptionID, key, plan, features,
	)
	if err != nil {
		return nil, fmt.Errorf("insert license key: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	return s.GetByID(id)
}

func (s *LicenseKeyStore) GetByID(id int64) (*model.LicenseKey, error) {
	row := s.db.QueryRow(`SELECT `+licenseKeyCols+` FROM license_keys WHERE id = ?`, id)
	lk, err := scanLicenseKey(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get license key: %w", err)
	}
	return lk, nil
}

func (s *LicenseKeyStore) GetByKey(key string) (*model.LicenseKey, error) {
	row := s.db.QueryRow(`SELECT `+licenseKeyCols+` FROM license_keys WHERE key = ?`, key)
	lk, err := scanLicenseKey(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get license key by key: %w", err)
	}
	return lk, nil
}

func (s *LicenseKeyStore) GetBySubscriptionID(subscriptionID int64) (*model.LicenseKey, error) {
	row := s.db.QueryRow(
		`SELECT `+licenseKeyCols+` FROM license_keys WHERE subscription_id = ? ORDER BY created_at DESC LIMIT 1`,
		subscriptionID,
	)
	lk, err := scanLicenseKey(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get license key by subscription: %w", err)
	}
	return lk, nil
}

func (s *LicenseKeyStore) Activate(id int64) error {
	_, err := s.db.Exec(
		`UPDATE license_keys SET activated_at = datetime('now') WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("activate license key: %w", err)
	}
	return nil
}

func (s *LicenseKeyStore) UpdateExpiry(id int64, expiresAt time.Time) error {
	_, err := s.db.Exec(
		`UPDATE license_keys SET expires_at = ? WHERE id = ?`,
		expiresAt, id,
	)
	if err != nil {
		return fmt.Errorf("update license key expiry: %w", err)
	}
	return nil
}

func (s *LicenseKeyStore) Revoke(id int64) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`UPDATE license_keys SET expires_at = ? WHERE id = ?`,
		now, id,
	)
	if err != nil {
		return fmt.Errorf("revoke license key: %w", err)
	}
	return nil
}

func (s *LicenseKeyStore) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM license_keys WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete license key: %w", err)
	}
	return nil
}
