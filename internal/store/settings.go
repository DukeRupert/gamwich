package store

import (
	"database/sql"
	"fmt"
	"time"
)

var kioskKeys = []string{
	"idle_timeout_minutes",
	"quiet_hours_enabled",
	"quiet_hours_start",
	"quiet_hours_end",
	"burn_in_prevention",
}

var weatherKeys = []string{
	"weather_latitude",
	"weather_longitude",
	"weather_units",
}

var themeKeys = []string{
	"theme_mode",
	"theme_selected",
	"theme_light",
	"theme_dark",
}

var tunnelKeys = []string{
	"tunnel_enabled",
	"tunnel_token",
}

var backupKeys = []string{
	"backup_enabled",
	"backup_schedule_hour",
	"backup_retention_days",
	"backup_passphrase_salt",
	"backup_passphrase_hash",
}

var emailKeys = []string{
	"email_postmark_token",
	"email_from_address",
	"email_base_url",
}

var s3Keys = []string{
	"backup_s3_endpoint",
	"backup_s3_bucket",
	"backup_s3_region",
	"backup_s3_access_key",
	"backup_s3_secret_key",
}

var vapidKeys = []string{
	"vapid_public_key",
	"vapid_private_key",
}

type SettingsStore struct {
	db *sql.DB
}

func NewSettingsStore(db *sql.DB) *SettingsStore {
	return &SettingsStore{db: db}
}

func (s *SettingsStore) Get(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("setting %q not found", key)
	}
	if err != nil {
		return "", fmt.Errorf("get setting %q: %w", key, err)
	}
	return value, nil
}

func (s *SettingsStore) GetAll() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT key, value FROM settings ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("get all settings: %w", err)
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		settings[key] = value
	}
	return settings, rows.Err()
}

func (s *SettingsStore) Set(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO settings (household_id, key, value, updated_at) VALUES (1, ?, ?, ?)
		 ON CONFLICT(household_id, key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}

func (s *SettingsStore) GetKioskSettings() (map[string]string, error) {
	settings := make(map[string]string)
	for _, key := range kioskKeys {
		var value string
		err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get kiosk setting %q: %w", key, err)
		}
		settings[key] = value
	}
	return settings, nil
}

func (s *SettingsStore) GetWeatherSettings() (map[string]string, error) {
	settings := make(map[string]string)
	for _, key := range weatherKeys {
		var value string
		err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get weather setting %q: %w", key, err)
		}
		settings[key] = value
	}
	return settings, nil
}

func (s *SettingsStore) GetThemeSettings() (map[string]string, error) {
	settings := make(map[string]string)
	for _, key := range themeKeys {
		var value string
		err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get theme setting %q: %w", key, err)
		}
		settings[key] = value
	}
	return settings, nil
}

func (s *SettingsStore) GetTunnelSettings() (map[string]string, error) {
	settings := make(map[string]string)
	for _, key := range tunnelKeys {
		var value string
		err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get tunnel setting %q: %w", key, err)
		}
		settings[key] = value
	}
	return settings, nil
}

func (s *SettingsStore) GetBackupSettings() (map[string]string, error) {
	settings := make(map[string]string)
	for _, key := range backupKeys {
		var value string
		err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get backup setting %q: %w", key, err)
		}
		settings[key] = value
	}
	return settings, nil
}

func (s *SettingsStore) GetEmailSettings() (map[string]string, error) {
	settings := make(map[string]string)
	for _, key := range emailKeys {
		var value string
		err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get email setting %q: %w", key, err)
		}
		settings[key] = value
	}
	return settings, nil
}

func (s *SettingsStore) GetS3Settings() (map[string]string, error) {
	settings := make(map[string]string)
	for _, key := range s3Keys {
		var value string
		err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get s3 setting %q: %w", key, err)
		}
		settings[key] = value
	}
	return settings, nil
}

func (s *SettingsStore) GetVAPIDSettings() (map[string]string, error) {
	settings := make(map[string]string)
	for _, key := range vapidKeys {
		var value string
		err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get vapid setting %q: %w", key, err)
		}
		settings[key] = value
	}
	return settings, nil
}
