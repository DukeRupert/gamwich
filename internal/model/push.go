package model

import "time"

// Notification type constants
const (
	NotifTypeCalendarReminder = "calendar_reminder"
	NotifTypeChoreDue         = "chore_due"
	NotifTypeGroceryAdded     = "grocery_added"
)

type PushSubscription struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	HouseholdID int64     `json:"household_id"`
	Endpoint    string    `json:"endpoint"`
	P256dhKey   string    `json:"p256dh_key"`
	AuthKey     string    `json:"auth_key"`
	DeviceName  string    `json:"device_name"`
	CreatedAt   time.Time `json:"created_at"`
}

type NotificationPreference struct {
	ID               int64     `json:"id"`
	UserID           int64     `json:"user_id"`
	HouseholdID      int64     `json:"household_id"`
	NotificationType string    `json:"notification_type"`
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
