package store

import (
	"strings"
	"testing"
	"time"

	"github.com/dukerupert/gamwich/internal/billing/database"
)

func setupLicenseKeyTestDB(t *testing.T) (*LicenseKeyStore, *SubscriptionStore, *AccountStore) {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewLicenseKeyStore(db), NewSubscriptionStore(db), NewAccountStore(db)
}

func TestLicenseKeyCreate(t *testing.T) {
	lks, ss, as := setupLicenseKeyTestDB(t)

	a, _ := as.Create("alice@example.com")
	sub, _ := ss.Create(a.ID, "cloud")

	lk, err := lks.Create(a.ID, sub.ID, "cloud", "tunnel,backup,push")
	if err != nil {
		t.Fatalf("create license key: %v", err)
	}
	if !strings.HasPrefix(lk.Key, "GW-") {
		t.Errorf("key %q does not start with GW-", lk.Key)
	}
	// Format: GW-XXXX-XXXX-XXXX-XXXX (22 chars)
	if len(lk.Key) != 22 {
		t.Errorf("key length = %d, want 22", len(lk.Key))
	}
	if lk.Plan != "cloud" {
		t.Errorf("plan = %q, want %q", lk.Plan, "cloud")
	}
	if lk.Features != "tunnel,backup,push" {
		t.Errorf("features = %q, want %q", lk.Features, "tunnel,backup,push")
	}
}

func TestLicenseKeyGetByKey(t *testing.T) {
	lks, ss, as := setupLicenseKeyTestDB(t)

	a, _ := as.Create("alice@example.com")
	sub, _ := ss.Create(a.ID, "cloud")
	created, _ := lks.Create(a.ID, sub.ID, "cloud", "tunnel")

	lk, err := lks.GetByKey(created.Key)
	if err != nil {
		t.Fatalf("get by key: %v", err)
	}
	if lk == nil {
		t.Fatal("expected license key, got nil")
	}
	if lk.ID != created.ID {
		t.Errorf("id = %d, want %d", lk.ID, created.ID)
	}
}

func TestLicenseKeyGetByKeyNotFound(t *testing.T) {
	lks, _, _ := setupLicenseKeyTestDB(t)

	lk, err := lks.GetByKey("GW-0000-0000-0000-0000")
	if err != nil {
		t.Fatalf("get by key: %v", err)
	}
	if lk != nil {
		t.Error("expected nil for nonexistent key")
	}
}

func TestLicenseKeyGetBySubscriptionID(t *testing.T) {
	lks, ss, as := setupLicenseKeyTestDB(t)

	a, _ := as.Create("alice@example.com")
	sub, _ := ss.Create(a.ID, "cloud")
	created, _ := lks.Create(a.ID, sub.ID, "cloud", "tunnel")

	lk, err := lks.GetBySubscriptionID(sub.ID)
	if err != nil {
		t.Fatalf("get by subscription id: %v", err)
	}
	if lk == nil {
		t.Fatal("expected license key, got nil")
	}
	if lk.ID != created.ID {
		t.Errorf("id = %d, want %d", lk.ID, created.ID)
	}
}

func TestLicenseKeyActivate(t *testing.T) {
	lks, ss, as := setupLicenseKeyTestDB(t)

	a, _ := as.Create("alice@example.com")
	sub, _ := ss.Create(a.ID, "cloud")
	created, _ := lks.Create(a.ID, sub.ID, "cloud", "tunnel")

	if created.ActivatedAt != nil {
		t.Error("expected nil activated_at before activation")
	}

	if err := lks.Activate(created.ID); err != nil {
		t.Fatalf("activate: %v", err)
	}

	lk, _ := lks.GetByID(created.ID)
	if lk.ActivatedAt == nil {
		t.Error("expected non-nil activated_at after activation")
	}
}

func TestLicenseKeyUpdateExpiry(t *testing.T) {
	lks, ss, as := setupLicenseKeyTestDB(t)

	a, _ := as.Create("alice@example.com")
	sub, _ := ss.Create(a.ID, "cloud")
	created, _ := lks.Create(a.ID, sub.ID, "cloud", "tunnel")

	expiry := time.Now().UTC().Add(30 * 24 * time.Hour)
	if err := lks.UpdateExpiry(created.ID, expiry); err != nil {
		t.Fatalf("update expiry: %v", err)
	}

	lk, _ := lks.GetByID(created.ID)
	if lk.ExpiresAt == nil {
		t.Fatal("expected non-nil expires_at")
	}
	if lk.ExpiresAt.Before(expiry.Add(-time.Second)) || lk.ExpiresAt.After(expiry.Add(time.Second)) {
		t.Errorf("expires_at = %v, want ~%v", lk.ExpiresAt, expiry)
	}
}

func TestLicenseKeyRevoke(t *testing.T) {
	lks, ss, as := setupLicenseKeyTestDB(t)

	a, _ := as.Create("alice@example.com")
	sub, _ := ss.Create(a.ID, "cloud")
	created, _ := lks.Create(a.ID, sub.ID, "cloud", "tunnel")

	if err := lks.Revoke(created.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	lk, _ := lks.GetByID(created.ID)
	if lk.ExpiresAt == nil {
		t.Fatal("expected non-nil expires_at after revoke")
	}
	if lk.ExpiresAt.After(time.Now().UTC().Add(time.Minute)) {
		t.Error("expected expires_at to be in the past or very near now")
	}
}

func TestLicenseKeyDelete(t *testing.T) {
	lks, ss, as := setupLicenseKeyTestDB(t)

	a, _ := as.Create("alice@example.com")
	sub, _ := ss.Create(a.ID, "cloud")
	created, _ := lks.Create(a.ID, sub.ID, "cloud", "tunnel")

	if err := lks.Delete(created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	lk, err := lks.GetByID(created.ID)
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if lk != nil {
		t.Error("expected nil after delete")
	}
}

func TestLicenseKeyUniqueKey(t *testing.T) {
	lks, ss, as := setupLicenseKeyTestDB(t)

	a, _ := as.Create("alice@example.com")
	sub, _ := ss.Create(a.ID, "cloud")

	// Create two keys and verify they're different
	lk1, _ := lks.Create(a.ID, sub.ID, "cloud", "tunnel")
	lk2, _ := lks.Create(a.ID, sub.ID, "cloud", "tunnel")

	if lk1.Key == lk2.Key {
		t.Error("expected unique keys, got identical")
	}
}
