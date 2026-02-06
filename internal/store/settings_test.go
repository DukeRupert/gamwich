package store

import (
	"testing"

	"github.com/dukerupert/gamwich/internal/database"
)

func setupSettingsTestDB(t *testing.T) *SettingsStore {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewSettingsStore(db)
}

func TestSettingsSeedData(t *testing.T) {
	ss := setupSettingsTestDB(t)

	settings, err := ss.GetKioskSettings()
	if err != nil {
		t.Fatalf("get kiosk settings: %v", err)
	}

	expected := map[string]string{
		"idle_timeout_minutes": "5",
		"quiet_hours_enabled":  "false",
		"quiet_hours_start":    "22:00",
		"quiet_hours_end":      "06:00",
		"burn_in_prevention":   "true",
	}

	for key, want := range expected {
		got, ok := settings[key]
		if !ok {
			t.Errorf("missing kiosk setting %q", key)
			continue
		}
		if got != want {
			t.Errorf("setting %q = %q, want %q", key, got, want)
		}
	}
}

func TestSettingsGet(t *testing.T) {
	ss := setupSettingsTestDB(t)

	val, err := ss.Get("idle_timeout_minutes")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if val != "5" {
		t.Errorf("idle_timeout_minutes = %q, want %q", val, "5")
	}
}

func TestSettingsGetNotFound(t *testing.T) {
	ss := setupSettingsTestDB(t)

	_, err := ss.Get("nonexistent_key")
	if err == nil {
		t.Fatal("expected error for nonexistent key, got nil")
	}
}

func TestSettingsSet(t *testing.T) {
	ss := setupSettingsTestDB(t)

	// Update existing
	if err := ss.Set("idle_timeout_minutes", "10"); err != nil {
		t.Fatalf("set: %v", err)
	}

	val, err := ss.Get("idle_timeout_minutes")
	if err != nil {
		t.Fatalf("get after set: %v", err)
	}
	if val != "10" {
		t.Errorf("idle_timeout_minutes = %q, want %q", val, "10")
	}

	// Insert new
	if err := ss.Set("custom_key", "custom_value"); err != nil {
		t.Fatalf("set new key: %v", err)
	}

	val, err = ss.Get("custom_key")
	if err != nil {
		t.Fatalf("get new key: %v", err)
	}
	if val != "custom_value" {
		t.Errorf("custom_key = %q, want %q", val, "custom_value")
	}
}

func TestSettingsGetAll(t *testing.T) {
	ss := setupSettingsTestDB(t)

	all, err := ss.GetAll()
	if err != nil {
		t.Fatalf("get all: %v", err)
	}

	// Should have at least the 5 seed settings
	if len(all) < 5 {
		t.Fatalf("expected at least 5 settings, got %d", len(all))
	}

	if all["idle_timeout_minutes"] != "5" {
		t.Errorf("idle_timeout_minutes = %q, want %q", all["idle_timeout_minutes"], "5")
	}
}

func TestSettingsGetKioskSettings(t *testing.T) {
	ss := setupSettingsTestDB(t)

	// Add a non-kiosk setting
	if err := ss.Set("non_kiosk_key", "value"); err != nil {
		t.Fatalf("set: %v", err)
	}

	kiosk, err := ss.GetKioskSettings()
	if err != nil {
		t.Fatalf("get kiosk: %v", err)
	}

	if len(kiosk) != 5 {
		t.Fatalf("expected 5 kiosk settings, got %d", len(kiosk))
	}

	if _, ok := kiosk["non_kiosk_key"]; ok {
		t.Error("non-kiosk key should not be in kiosk settings")
	}
}
