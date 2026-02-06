package store

import (
	"testing"

	"github.com/dukerupert/gamwich/internal/database"
)

func setupMagicLinkTestDB(t *testing.T) *MagicLinkStore {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewMagicLinkStore(db)
}

func TestMagicLinkCreate(t *testing.T) {
	ms := setupMagicLinkTestDB(t)

	ml, err := ms.Create("alice@example.com", "login", nil)
	if err != nil {
		t.Fatalf("create magic link: %v", err)
	}
	if ml.Token == "" {
		t.Error("expected non-empty token")
	}
	if len(ml.Token) != 64 {
		t.Errorf("token length = %d, want 64", len(ml.Token))
	}
	if ml.Email != "alice@example.com" {
		t.Errorf("email = %q, want %q", ml.Email, "alice@example.com")
	}
	if ml.Purpose != "login" {
		t.Errorf("purpose = %q, want %q", ml.Purpose, "login")
	}
	if ml.HouseholdID != nil {
		t.Errorf("household_id = %v, want nil", ml.HouseholdID)
	}
}

func TestMagicLinkCreateWithHousehold(t *testing.T) {
	ms := setupMagicLinkTestDB(t)

	hID := int64(1) // default household from migration
	ml, err := ms.Create("alice@example.com", "invite", &hID)
	if err != nil {
		t.Fatalf("create magic link: %v", err)
	}
	if ml.HouseholdID == nil || *ml.HouseholdID != 1 {
		t.Errorf("household_id = %v, want 1", ml.HouseholdID)
	}
}

func TestMagicLinkGetByToken(t *testing.T) {
	ms := setupMagicLinkTestDB(t)

	created, _ := ms.Create("alice@example.com", "login", nil)

	ml, err := ms.GetByToken(created.Token)
	if err != nil {
		t.Fatalf("get by token: %v", err)
	}
	if ml == nil {
		t.Fatal("expected magic link, got nil")
	}
	if ml.ID != created.ID {
		t.Errorf("id = %d, want %d", ml.ID, created.ID)
	}
}

func TestMagicLinkGetByTokenNotFound(t *testing.T) {
	ms := setupMagicLinkTestDB(t)

	ml, err := ms.GetByToken("nonexistent")
	if err != nil {
		t.Fatalf("get by token: %v", err)
	}
	if ml != nil {
		t.Error("expected nil for nonexistent token")
	}
}

func TestMagicLinkMarkUsed(t *testing.T) {
	ms := setupMagicLinkTestDB(t)

	created, _ := ms.Create("alice@example.com", "login", nil)

	if err := ms.MarkUsed(created.ID); err != nil {
		t.Fatalf("mark used: %v", err)
	}

	// Should not be returned after marking as used
	ml, err := ms.GetByToken(created.Token)
	if err != nil {
		t.Fatalf("get after mark used: %v", err)
	}
	if ml != nil {
		t.Error("expected nil for used magic link")
	}
}

func TestMagicLinkDeleteExpired(t *testing.T) {
	ms := setupMagicLinkTestDB(t)

	// Create a link and manually expire it
	created, _ := ms.Create("alice@example.com", "login", nil)
	ms.db.Exec(`UPDATE magic_links SET expires_at = datetime('now', '-1 hour') WHERE id = ?`, created.ID)

	count, err := ms.DeleteExpired()
	if err != nil {
		t.Fatalf("delete expired: %v", err)
	}
	if count != 1 {
		t.Errorf("deleted = %d, want 1", count)
	}
}
