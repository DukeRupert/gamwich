package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/dukerupert/gamwich/internal/model"
)

type PushStore struct {
	db *sql.DB
}

func NewPushStore(db *sql.DB) *PushStore {
	return &PushStore{db: db}
}

func (s *PushStore) CreateSubscription(userID, householdID int64, endpoint, p256dh, auth, deviceName string) (*model.PushSubscription, error) {
	result, err := s.db.Exec(
		`INSERT INTO push_subscriptions (user_id, household_id, endpoint, p256dh_key, auth_key, device_name)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(endpoint) DO UPDATE SET p256dh_key = excluded.p256dh_key, auth_key = excluded.auth_key, device_name = excluded.device_name`,
		userID, householdID, endpoint, p256dh, auth, deviceName,
	)
	if err != nil {
		return nil, fmt.Errorf("create push subscription: %w", err)
	}
	id, _ := result.LastInsertId()

	// LastInsertId may be 0 on conflict update; re-query by endpoint
	if id == 0 {
		return s.getByEndpoint(endpoint)
	}
	return s.GetByID(id, householdID)
}

func (s *PushStore) GetByID(id, householdID int64) (*model.PushSubscription, error) {
	var sub model.PushSubscription
	err := s.db.QueryRow(
		`SELECT id, user_id, household_id, endpoint, p256dh_key, auth_key, device_name, created_at
		 FROM push_subscriptions WHERE id = ? AND household_id = ?`, id, householdID,
	).Scan(&sub.ID, &sub.UserID, &sub.HouseholdID, &sub.Endpoint, &sub.P256dhKey, &sub.AuthKey, &sub.DeviceName, &sub.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get push subscription: %w", err)
	}
	return &sub, nil
}

func (s *PushStore) getByEndpoint(endpoint string) (*model.PushSubscription, error) {
	var sub model.PushSubscription
	err := s.db.QueryRow(
		`SELECT id, user_id, household_id, endpoint, p256dh_key, auth_key, device_name, created_at
		 FROM push_subscriptions WHERE endpoint = ?`, endpoint,
	).Scan(&sub.ID, &sub.UserID, &sub.HouseholdID, &sub.Endpoint, &sub.P256dhKey, &sub.AuthKey, &sub.DeviceName, &sub.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get push subscription by endpoint: %w", err)
	}
	return &sub, nil
}

func (s *PushStore) ListByUser(userID, householdID int64) ([]model.PushSubscription, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, household_id, endpoint, p256dh_key, auth_key, device_name, created_at
		 FROM push_subscriptions WHERE user_id = ? AND household_id = ? ORDER BY created_at DESC`,
		userID, householdID,
	)
	if err != nil {
		return nil, fmt.Errorf("list push subscriptions by user: %w", err)
	}
	defer rows.Close()
	return scanSubscriptions(rows)
}

func (s *PushStore) ListByHousehold(householdID int64) ([]model.PushSubscription, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, household_id, endpoint, p256dh_key, auth_key, device_name, created_at
		 FROM push_subscriptions WHERE household_id = ? ORDER BY created_at DESC`,
		householdID,
	)
	if err != nil {
		return nil, fmt.Errorf("list push subscriptions by household: %w", err)
	}
	defer rows.Close()
	return scanSubscriptions(rows)
}

func (s *PushStore) DeleteSubscription(id, householdID int64) error {
	_, err := s.db.Exec(`DELETE FROM push_subscriptions WHERE id = ? AND household_id = ?`, id, householdID)
	if err != nil {
		return fmt.Errorf("delete push subscription: %w", err)
	}
	return nil
}

func (s *PushStore) DeleteByEndpoint(endpoint string) error {
	_, err := s.db.Exec(`DELETE FROM push_subscriptions WHERE endpoint = ?`, endpoint)
	if err != nil {
		return fmt.Errorf("delete push subscription by endpoint: %w", err)
	}
	return nil
}

// ListHouseholdIDs returns distinct household IDs that have push subscriptions.
func (s *PushStore) ListHouseholdIDs() ([]int64, error) {
	rows, err := s.db.Query(`SELECT DISTINCT household_id FROM push_subscriptions`)
	if err != nil {
		return nil, fmt.Errorf("list push household ids: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan household id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetPreferences returns notification preferences for a user in a household.
func (s *PushStore) GetPreferences(userID, householdID int64) ([]model.NotificationPreference, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, household_id, notification_type, enabled, created_at, updated_at
		 FROM notification_preferences WHERE user_id = ? AND household_id = ?`,
		userID, householdID,
	)
	if err != nil {
		return nil, fmt.Errorf("get notification preferences: %w", err)
	}
	defer rows.Close()

	var prefs []model.NotificationPreference
	for rows.Next() {
		var p model.NotificationPreference
		var enabledInt int
		if err := rows.Scan(&p.ID, &p.UserID, &p.HouseholdID, &p.NotificationType, &enabledInt, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan notification preference: %w", err)
		}
		p.Enabled = enabledInt != 0
		prefs = append(prefs, p)
	}
	return prefs, rows.Err()
}

// SetPreference upserts a notification preference.
func (s *PushStore) SetPreference(userID, householdID int64, notifType string, enabled bool) error {
	var enabledInt int
	if enabled {
		enabledInt = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO notification_preferences (user_id, household_id, notification_type, enabled)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(user_id, household_id, notification_type) DO UPDATE SET enabled = excluded.enabled`,
		userID, householdID, notifType, enabledInt,
	)
	if err != nil {
		return fmt.Errorf("set notification preference: %w", err)
	}
	return nil
}

// IsPreferenceEnabled checks if a specific notification type is enabled for a user.
// Returns true by default if no preference record exists.
func (s *PushStore) IsPreferenceEnabled(userID, householdID int64, notifType string) (bool, error) {
	var enabledInt int
	err := s.db.QueryRow(
		`SELECT enabled FROM notification_preferences
		 WHERE user_id = ? AND household_id = ? AND notification_type = ?`,
		userID, householdID, notifType,
	).Scan(&enabledInt)
	if err == sql.ErrNoRows {
		return true, nil // default enabled
	}
	if err != nil {
		return false, fmt.Errorf("check notification preference: %w", err)
	}
	return enabledInt != 0, nil
}

// RecordSent records that a notification was sent (for dedup).
func (s *PushStore) RecordSent(householdID int64, notifType, refID string, leadTime int) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO sent_notifications (household_id, notification_type, reference_id, lead_time_minutes)
		 VALUES (?, ?, ?, ?)`,
		householdID, notifType, refID, leadTime,
	)
	if err != nil {
		return fmt.Errorf("record sent notification: %w", err)
	}
	return nil
}

// WasSent checks if a notification was already sent.
func (s *PushStore) WasSent(householdID int64, notifType, refID string, leadTime int) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM sent_notifications
		 WHERE household_id = ? AND notification_type = ? AND reference_id = ? AND lead_time_minutes = ?`,
		householdID, notifType, refID, leadTime,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check sent notification: %w", err)
	}
	return count > 0, nil
}

// CleanupSent deletes sent_notifications older than the given time.
func (s *PushStore) CleanupSent(before time.Time) error {
	_, err := s.db.Exec(`DELETE FROM sent_notifications WHERE sent_at < ?`, before.UTC())
	if err != nil {
		return fmt.Errorf("cleanup sent notifications: %w", err)
	}
	return nil
}

func scanSubscriptions(rows *sql.Rows) ([]model.PushSubscription, error) {
	var subs []model.PushSubscription
	for rows.Next() {
		var sub model.PushSubscription
		if err := rows.Scan(&sub.ID, &sub.UserID, &sub.HouseholdID, &sub.Endpoint, &sub.P256dhKey, &sub.AuthKey, &sub.DeviceName, &sub.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan push subscription: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}
