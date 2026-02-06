package store

import (
	"testing"

	"github.com/dukerupert/gamwich/internal/billing/database"
)

func setupSessionTestDB(t *testing.T) (*SessionStore, *AccountStore) {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewSessionStore(db), NewAccountStore(db)
}

func TestSessionCreate(t *testing.T) {
	ss, as := setupSessionTestDB(t)

	a, _ := as.Create("alice@example.com")
	sess, err := ss.Create(a.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if sess.Token == "" {
		t.Error("expected non-empty token")
	}
	if len(sess.Token) != 64 { // 32 bytes hex-encoded
		t.Errorf("token length = %d, want 64", len(sess.Token))
	}
	if sess.AccountID != a.ID {
		t.Errorf("account_id = %d, want %d", sess.AccountID, a.ID)
	}
}

func TestSessionGetByToken(t *testing.T) {
	ss, as := setupSessionTestDB(t)

	a, _ := as.Create("alice@example.com")
	created, _ := ss.Create(a.ID)

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
	ss, _ := setupSessionTestDB(t)

	sess, err := ss.GetByToken("nonexistent")
	if err != nil {
		t.Fatalf("get by token: %v", err)
	}
	if sess != nil {
		t.Error("expected nil for nonexistent token")
	}
}

func TestSessionDelete(t *testing.T) {
	ss, as := setupSessionTestDB(t)

	a, _ := as.Create("alice@example.com")
	created, _ := ss.Create(a.ID)

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

func TestSessionDeleteByAccountID(t *testing.T) {
	ss, as := setupSessionTestDB(t)

	a, _ := as.Create("alice@example.com")
	ss.Create(a.ID)
	ss.Create(a.ID)

	if err := ss.DeleteByAccountID(a.ID); err != nil {
		t.Fatalf("delete by account id: %v", err)
	}

	var count int
	ss.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE account_id = ?`, a.ID).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 sessions, got %d", count)
	}
}
