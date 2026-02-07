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

	// Should have at least the 22 seed settings (5 kiosk + 3 weather + 4 theme + 3 email + 5 s3 + 2 vapid)
	if len(all) < 22 {
		t.Fatalf("expected at least 22 settings, got %d", len(all))
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

func TestSettingsThemeSeedData(t *testing.T) {
	ss := setupSettingsTestDB(t)

	settings, err := ss.GetThemeSettings()
	if err != nil {
		t.Fatalf("get theme settings: %v", err)
	}

	expected := map[string]string{
		"theme_mode":     "manual",
		"theme_selected": "garden",
		"theme_light":    "garden",
		"theme_dark":     "forest",
	}

	for key, want := range expected {
		got, ok := settings[key]
		if !ok {
			t.Errorf("missing theme setting %q", key)
			continue
		}
		if got != want {
			t.Errorf("setting %q = %q, want %q", key, got, want)
		}
	}
}

func TestSettingsGetThemeSettings(t *testing.T) {
	ss := setupSettingsTestDB(t)

	// Add a non-theme setting
	if err := ss.Set("non_theme_key", "value"); err != nil {
		t.Fatalf("set: %v", err)
	}

	theme, err := ss.GetThemeSettings()
	if err != nil {
		t.Fatalf("get theme: %v", err)
	}

	if len(theme) != 4 {
		t.Fatalf("expected 4 theme settings, got %d", len(theme))
	}

	if _, ok := theme["non_theme_key"]; ok {
		t.Error("non-theme key should not be in theme settings")
	}
}

func TestSettingsEmailSeedData(t *testing.T) {
	ss := setupSettingsTestDB(t)

	settings, err := ss.GetEmailSettings()
	if err != nil {
		t.Fatalf("get email settings: %v", err)
	}

	for _, key := range []string{"email_postmark_token", "email_from_address", "email_base_url"} {
		got, ok := settings[key]
		if !ok {
			t.Errorf("missing email setting %q", key)
			continue
		}
		if got != "" {
			t.Errorf("setting %q = %q, want empty string", key, got)
		}
	}
}

func TestSettingsGetEmailSettings(t *testing.T) {
	ss := setupSettingsTestDB(t)

	if err := ss.Set("email_postmark_token", "test-token"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := ss.Set("email_from_address", "test@example.com"); err != nil {
		t.Fatalf("set: %v", err)
	}

	settings, err := ss.GetEmailSettings()
	if err != nil {
		t.Fatalf("get email: %v", err)
	}

	if settings["email_postmark_token"] != "test-token" {
		t.Errorf("email_postmark_token = %q, want %q", settings["email_postmark_token"], "test-token")
	}
	if settings["email_from_address"] != "test@example.com" {
		t.Errorf("email_from_address = %q, want %q", settings["email_from_address"], "test@example.com")
	}
	if len(settings) != 3 {
		t.Errorf("expected 3 email settings, got %d", len(settings))
	}
}

func TestSettingsS3SeedData(t *testing.T) {
	ss := setupSettingsTestDB(t)

	settings, err := ss.GetS3Settings()
	if err != nil {
		t.Fatalf("get s3 settings: %v", err)
	}

	for _, key := range []string{"backup_s3_endpoint", "backup_s3_bucket", "backup_s3_region", "backup_s3_access_key", "backup_s3_secret_key"} {
		got, ok := settings[key]
		if !ok {
			t.Errorf("missing s3 setting %q", key)
			continue
		}
		if got != "" {
			t.Errorf("setting %q = %q, want empty string", key, got)
		}
	}
}

func TestSettingsGetS3Settings(t *testing.T) {
	ss := setupSettingsTestDB(t)

	if err := ss.Set("backup_s3_bucket", "my-bucket"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := ss.Set("backup_s3_access_key", "AKID"); err != nil {
		t.Fatalf("set: %v", err)
	}

	settings, err := ss.GetS3Settings()
	if err != nil {
		t.Fatalf("get s3: %v", err)
	}

	if settings["backup_s3_bucket"] != "my-bucket" {
		t.Errorf("backup_s3_bucket = %q, want %q", settings["backup_s3_bucket"], "my-bucket")
	}
	if settings["backup_s3_access_key"] != "AKID" {
		t.Errorf("backup_s3_access_key = %q, want %q", settings["backup_s3_access_key"], "AKID")
	}
	if len(settings) != 5 {
		t.Errorf("expected 5 s3 settings, got %d", len(settings))
	}
}

func TestSettingsVAPIDSeedData(t *testing.T) {
	ss := setupSettingsTestDB(t)

	settings, err := ss.GetVAPIDSettings()
	if err != nil {
		t.Fatalf("get vapid settings: %v", err)
	}

	for _, key := range []string{"vapid_public_key", "vapid_private_key"} {
		got, ok := settings[key]
		if !ok {
			t.Errorf("missing vapid setting %q", key)
			continue
		}
		if got != "" {
			t.Errorf("setting %q = %q, want empty string", key, got)
		}
	}
}

func TestSettingsGetVAPIDSettings(t *testing.T) {
	ss := setupSettingsTestDB(t)

	if err := ss.Set("vapid_public_key", "pub-key-123"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := ss.Set("vapid_private_key", "priv-key-456"); err != nil {
		t.Fatalf("set: %v", err)
	}

	settings, err := ss.GetVAPIDSettings()
	if err != nil {
		t.Fatalf("get vapid: %v", err)
	}

	if settings["vapid_public_key"] != "pub-key-123" {
		t.Errorf("vapid_public_key = %q, want %q", settings["vapid_public_key"], "pub-key-123")
	}
	if settings["vapid_private_key"] != "priv-key-456" {
		t.Errorf("vapid_private_key = %q, want %q", settings["vapid_private_key"], "priv-key-456")
	}
	if len(settings) != 2 {
		t.Errorf("expected 2 vapid settings, got %d", len(settings))
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
