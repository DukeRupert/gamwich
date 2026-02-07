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
	if len(ml.Token) != 6 {
		t.Errorf("token length = %d, want 6", len(ml.Token))
	}
	// Verify it's all digits
	for _, c := range ml.Token {
		if c < '0' || c > '9' {
			t.Errorf("token contains non-digit: %c", c)
		}
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
	if ml.Attempts != 0 {
		t.Errorf("attempts = %d, want 0", ml.Attempts)
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

func TestMagicLinkCreateInvalidatesPrevious(t *testing.T) {
	ms := setupMagicLinkTestDB(t)

	first, err := ms.Create("alice@example.com", "login", nil)
	if err != nil {
		t.Fatalf("create first: %v", err)
	}

	// Create a second code for the same email
	second, err := ms.Create("alice@example.com", "login", nil)
	if err != nil {
		t.Fatalf("create second: %v", err)
	}

	// First code should be invalidated (used)
	ml, err := ms.GetByEmailAndCode("alice@example.com", first.Token)
	if err != nil {
		t.Fatalf("get first: %v", err)
	}
	if ml != nil {
		t.Error("expected first code to be invalidated")
	}

	// Second code should still be valid
	ml, err = ms.GetByEmailAndCode("alice@example.com", second.Token)
	if err != nil {
		t.Fatalf("get second: %v", err)
	}
	if ml == nil {
		t.Fatal("expected second code to be valid")
	}
}

func TestMagicLinkGetByEmailAndCode(t *testing.T) {
	ms := setupMagicLinkTestDB(t)

	created, _ := ms.Create("alice@example.com", "login", nil)

	ml, err := ms.GetByEmailAndCode("alice@example.com", created.Token)
	if err != nil {
		t.Fatalf("get by email and code: %v", err)
	}
	if ml == nil {
		t.Fatal("expected magic link, got nil")
	}
	if ml.ID != created.ID {
		t.Errorf("id = %d, want %d", ml.ID, created.ID)
	}
}

func TestMagicLinkGetByEmailAndCodeNotFound(t *testing.T) {
	ms := setupMagicLinkTestDB(t)

	ml, err := ms.GetByEmailAndCode("alice@example.com", "000000")
	if err != nil {
		t.Fatalf("get by email and code: %v", err)
	}
	if ml != nil {
		t.Error("expected nil for nonexistent code")
	}
}

func TestMagicLinkGetByEmailAndCodeWrongEmail(t *testing.T) {
	ms := setupMagicLinkTestDB(t)

	created, _ := ms.Create("alice@example.com", "login", nil)

	// Correct code but wrong email
	ml, err := ms.GetByEmailAndCode("bob@example.com", created.Token)
	if err != nil {
		t.Fatalf("get by email and code: %v", err)
	}
	if ml != nil {
		t.Error("expected nil for wrong email")
	}
}

func TestMagicLinkGetLatestByEmail(t *testing.T) {
	ms := setupMagicLinkTestDB(t)

	created, _ := ms.Create("alice@example.com", "login", nil)

	ml, err := ms.GetLatestByEmail("alice@example.com")
	if err != nil {
		t.Fatalf("get latest: %v", err)
	}
	if ml == nil {
		t.Fatal("expected magic link, got nil")
	}
	if ml.ID != created.ID {
		t.Errorf("id = %d, want %d", ml.ID, created.ID)
	}
}

func TestMagicLinkGetLatestByEmailNotFound(t *testing.T) {
	ms := setupMagicLinkTestDB(t)

	ml, err := ms.GetLatestByEmail("nobody@example.com")
	if err != nil {
		t.Fatalf("get latest: %v", err)
	}
	if ml != nil {
		t.Error("expected nil for nonexistent email")
	}
}

func TestMagicLinkIncrementAttempts(t *testing.T) {
	ms := setupMagicLinkTestDB(t)

	created, _ := ms.Create("alice@example.com", "login", nil)

	attempts, err := ms.IncrementAttempts(created.ID)
	if err != nil {
		t.Fatalf("increment attempts: %v", err)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}

	attempts, err = ms.IncrementAttempts(created.ID)
	if err != nil {
		t.Fatalf("increment attempts again: %v", err)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

func TestMagicLinkMarkUsed(t *testing.T) {
	ms := setupMagicLinkTestDB(t)

	created, _ := ms.Create("alice@example.com", "login", nil)

	if err := ms.MarkUsed(created.ID); err != nil {
		t.Fatalf("mark used: %v", err)
	}

	// Should not be returned after marking as used
	ml, err := ms.GetByEmailAndCode("alice@example.com", created.Token)
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
