package store

import (
	"testing"

	"github.com/dukerupert/gamwich/internal/database"
)

func setupUserTestDB(t *testing.T) *UserStore {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewUserStore(db)
}

func TestUserCreate(t *testing.T) {
	us := setupUserTestDB(t)

	u, err := us.Create("alice@example.com", "Alice")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if u.Email != "alice@example.com" {
		t.Errorf("email = %q, want %q", u.Email, "alice@example.com")
	}
	if u.Name != "Alice" {
		t.Errorf("name = %q, want %q", u.Name, "Alice")
	}
	if u.ID == 0 {
		t.Error("expected non-zero ID")
	}
}

func TestUserCreateDuplicateEmail(t *testing.T) {
	us := setupUserTestDB(t)

	if _, err := us.Create("alice@example.com", "Alice"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := us.Create("alice@example.com", "Alice2"); err == nil {
		t.Fatal("expected error for duplicate email, got nil")
	}
}

func TestUserGetByID(t *testing.T) {
	us := setupUserTestDB(t)

	created, err := us.Create("alice@example.com", "Alice")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	u, err := us.GetByID(created.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if u.Email != "alice@example.com" {
		t.Errorf("email = %q, want %q", u.Email, "alice@example.com")
	}
}

func TestUserGetByIDNotFound(t *testing.T) {
	us := setupUserTestDB(t)

	u, err := us.GetByID(999)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if u != nil {
		t.Error("expected nil for nonexistent user")
	}
}

func TestUserGetByEmail(t *testing.T) {
	us := setupUserTestDB(t)

	if _, err := us.Create("alice@example.com", "Alice"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	u, err := us.GetByEmail("alice@example.com")
	if err != nil {
		t.Fatalf("get by email: %v", err)
	}
	if u == nil {
		t.Fatal("expected user, got nil")
	}
	if u.Name != "Alice" {
		t.Errorf("name = %q, want %q", u.Name, "Alice")
	}
}

func TestUserGetByEmailNotFound(t *testing.T) {
	us := setupUserTestDB(t)

	u, err := us.GetByEmail("nobody@example.com")
	if err != nil {
		t.Fatalf("get by email: %v", err)
	}
	if u != nil {
		t.Error("expected nil for nonexistent email")
	}
}

func TestUserUpdate(t *testing.T) {
	us := setupUserTestDB(t)

	created, err := us.Create("alice@example.com", "Alice")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	updated, err := us.Update(created.ID, "alice2@example.com", "Alice Updated")
	if err != nil {
		t.Fatalf("update user: %v", err)
	}
	if updated.Email != "alice2@example.com" {
		t.Errorf("email = %q, want %q", updated.Email, "alice2@example.com")
	}
	if updated.Name != "Alice Updated" {
		t.Errorf("name = %q, want %q", updated.Name, "Alice Updated")
	}
}

func TestUserDelete(t *testing.T) {
	us := setupUserTestDB(t)

	created, err := us.Create("alice@example.com", "Alice")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := us.Delete(created.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	u, err := us.GetByID(created.ID)
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if u != nil {
		t.Error("expected nil after delete")
	}
}
