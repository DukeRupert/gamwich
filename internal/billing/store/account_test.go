package store

import (
	"testing"

	"github.com/dukerupert/gamwich/internal/billing/database"
)

func setupAccountTestDB(t *testing.T) *AccountStore {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewAccountStore(db)
}

func TestAccountCreate(t *testing.T) {
	s := setupAccountTestDB(t)

	a, err := s.Create("alice@example.com")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if a.Email != "alice@example.com" {
		t.Errorf("email = %q, want %q", a.Email, "alice@example.com")
	}
	if a.StripeCustomerID != nil {
		t.Error("expected nil stripe customer id")
	}
}

func TestAccountGetByID(t *testing.T) {
	s := setupAccountTestDB(t)

	created, _ := s.Create("alice@example.com")
	a, err := s.GetByID(created.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if a == nil {
		t.Fatal("expected account, got nil")
	}
	if a.ID != created.ID {
		t.Errorf("id = %d, want %d", a.ID, created.ID)
	}
}

func TestAccountGetByIDNotFound(t *testing.T) {
	s := setupAccountTestDB(t)

	a, err := s.GetByID(999)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if a != nil {
		t.Error("expected nil for nonexistent id")
	}
}

func TestAccountGetByEmail(t *testing.T) {
	s := setupAccountTestDB(t)

	created, _ := s.Create("alice@example.com")
	a, err := s.GetByEmail("alice@example.com")
	if err != nil {
		t.Fatalf("get by email: %v", err)
	}
	if a == nil {
		t.Fatal("expected account, got nil")
	}
	if a.ID != created.ID {
		t.Errorf("id = %d, want %d", a.ID, created.ID)
	}
}

func TestAccountGetByEmailNotFound(t *testing.T) {
	s := setupAccountTestDB(t)

	a, err := s.GetByEmail("nonexistent@example.com")
	if err != nil {
		t.Fatalf("get by email: %v", err)
	}
	if a != nil {
		t.Error("expected nil for nonexistent email")
	}
}

func TestAccountUpdateStripeCustomerID(t *testing.T) {
	s := setupAccountTestDB(t)

	created, _ := s.Create("alice@example.com")
	if err := s.UpdateStripeCustomerID(created.ID, "cus_123"); err != nil {
		t.Fatalf("update stripe id: %v", err)
	}

	a, _ := s.GetByID(created.ID)
	if a.StripeCustomerID == nil || *a.StripeCustomerID != "cus_123" {
		t.Errorf("stripe_customer_id = %v, want %q", a.StripeCustomerID, "cus_123")
	}
}

func TestAccountDelete(t *testing.T) {
	s := setupAccountTestDB(t)

	created, _ := s.Create("alice@example.com")
	if err := s.Delete(created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	a, err := s.GetByID(created.ID)
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if a != nil {
		t.Error("expected nil after delete")
	}
}

func TestAccountDuplicateEmail(t *testing.T) {
	s := setupAccountTestDB(t)

	_, err := s.Create("alice@example.com")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err = s.Create("alice@example.com")
	if err == nil {
		t.Error("expected error for duplicate email")
	}
}
