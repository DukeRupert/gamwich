package store

import (
	"testing"

	"github.com/dukerupert/gamwich/internal/database"
)

func setupSessionTestDB(t *testing.T) (*SessionStore, *UserStore, *HouseholdStore) {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewSessionStore(db), NewUserStore(db), NewHouseholdStore(db)
}

func TestSessionCreate(t *testing.T) {
	ss, us, _ := setupSessionTestDB(t)

	u, err := us.Create("alice@example.com", "Alice")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	sess, err := ss.Create(u.ID, 1) // default household
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if sess.Token == "" {
		t.Error("expected non-empty token")
	}
	if len(sess.Token) != 64 { // 32 bytes hex-encoded
		t.Errorf("token length = %d, want 64", len(sess.Token))
	}
	if sess.UserID != u.ID {
		t.Errorf("user_id = %d, want %d", sess.UserID, u.ID)
	}
	if sess.HouseholdID != 1 {
		t.Errorf("household_id = %d, want 1", sess.HouseholdID)
	}
}

func TestSessionGetByToken(t *testing.T) {
	ss, us, _ := setupSessionTestDB(t)

	u, _ := us.Create("alice@example.com", "Alice")
	created, _ := ss.Create(u.ID, 1)

	sess, err := ss.GetByToken(created.Token)
	if err != nil {
		t.Fatalf("get by token: %v", err)
	}
	if sess == nil {
		t.Fatal("expected session, got nil")
	}
	if sess.ID != created.ID {
		t.Errorf("id = %d, want %d", sess.ID, created.ID)
	}
}

func TestSessionGetByTokenNotFound(t *testing.T) {
	ss, _, _ := setupSessionTestDB(t)

	sess, err := ss.GetByToken("nonexistent")
	if err != nil {
		t.Fatalf("get by token: %v", err)
	}
	if sess != nil {
		t.Error("expected nil for nonexistent token")
	}
}

func TestSessionDelete(t *testing.T) {
	ss, us, _ := setupSessionTestDB(t)

	u, _ := us.Create("alice@example.com", "Alice")
	created, _ := ss.Create(u.ID, 1)

	if err := ss.Delete(created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	sess, err := ss.GetByToken(created.Token)
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if sess != nil {
		t.Error("expected nil after delete")
	}
}

func TestSessionDeleteByUserID(t *testing.T) {
	ss, us, _ := setupSessionTestDB(t)

	u, _ := us.Create("alice@example.com", "Alice")
	ss.Create(u.ID, 1)
	ss.Create(u.ID, 1)

	if err := ss.DeleteByUserID(u.ID); err != nil {
		t.Fatalf("delete by user id: %v", err)
	}

	// Both sessions should be gone
	var count int
	ss.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE user_id = ?`, u.ID).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 sessions, got %d", count)
	}
}

func TestSessionUpdateHouseholdID(t *testing.T) {
	ss, us, hs := setupSessionTestDB(t)

	u, _ := us.Create("alice@example.com", "Alice")
	h2, _ := hs.Create("Second Household")
	created, _ := ss.Create(u.ID, 1)

	if err := ss.UpdateHouseholdID(created.ID, h2.ID); err != nil {
		t.Fatalf("update household id: %v", err)
	}

	sess, err := ss.GetByToken(created.Token)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if sess.HouseholdID != h2.ID {
		t.Errorf("household_id = %d, want %d", sess.HouseholdID, h2.ID)
	}
}
