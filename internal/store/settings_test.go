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

	// Should have at least the 8 seed settings (5 kiosk + 3 weather)
	if len(all) < 8 {
		t.Fatalf("expected at least 8 settings, got %d", len(all))
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

func TestSettingsWeatherSeedData(t *testing.T) {
	ss := setupSettingsTestDB(t)

	settings, err := ss.GetWeatherSettings()
	if err != nil {
		t.Fatalf("get weather settings: %v", err)
	}

	expected := map[string]string{
		"weather_latitude":  "",
		"weather_longitude": "",
		"weather_units":     "fahrenheit",
	}

	for key, want := range expected {
		got, ok := settings[key]
		if !ok {
			t.Errorf("missing weather setting %q", key)
			continue
		}
		if got != want {
			t.Errorf("setting %q = %q, want %q", key, got, want)
		}
	}
}

func TestSettingsGetWeatherSettings(t *testing.T) {
	ss := setupSettingsTestDB(t)

	// Add a non-weather setting
	if err := ss.Set("non_weather_key", "value"); err != nil {
		t.Fatalf("set: %v", err)
	}

	weather, err := ss.GetWeatherSettings()
	if err != nil {
		t.Fatalf("get weather: %v", err)
	}

	if len(weather) != 3 {
		t.Fatalf("expected 3 weather settings, got %d", len(weather))
	}

	if _, ok := weather["non_weather_key"]; ok {
		t.Error("non-weather key should not be in weather settings")
	}
}

func TestSettingsWeatherUpdate(t *testing.T) {
	ss := setupSettingsTestDB(t)

	if err := ss.Set("weather_latitude", "47.6062"); err != nil {
		t.Fatalf("set lat: %v", err)
	}
	if err := ss.Set("weather_longitude", "-122.3321"); err != nil {
		t.Fatalf("set lon: %v", err)
	}
	if err := ss.Set("weather_units", "celsius"); err != nil {
		t.Fatalf("set units: %v", err)
	}

	settings, err := ss.GetWeatherSettings()
	if err != nil {
		t.Fatalf("get weather: %v", err)
	}

	if settings["weather_latitude"] != "47.6062" {
		t.Errorf("latitude = %q, want %q", settings["weather_latitude"], "47.6062")
	}
	if settings["weather_longitude"] != "-122.3321" {
		t.Errorf("longitude = %q, want %q", settings["weather_longitude"], "-122.3321")
	}
	if settings["weather_units"] != "celsius" {
		t.Errorf("units = %q, want %q", settings["weather_units"], "celsius")
	}
}
