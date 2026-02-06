package store

import (
	"testing"

	"github.com/dukerupert/gamwich/internal/billing/database"
)

func setupSubscriptionTestDB(t *testing.T) (*SubscriptionStore, *AccountStore) {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewSubscriptionStore(db), NewAccountStore(db)
}

func TestSubscriptionCreate(t *testing.T) {
	ss, as := setupSubscriptionTestDB(t)

	a, _ := as.Create("alice@example.com")
	sub, err := ss.Create(a.ID, "cloud")
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	if sub.AccountID != a.ID {
		t.Errorf("account_id = %d, want %d", sub.AccountID, a.ID)
	}
	if sub.Plan != "cloud" {
		t.Errorf("plan = %q, want %q", sub.Plan, "cloud")
	}
	if sub.Status != "active" {
		t.Errorf("status = %q, want %q", sub.Status, "active")
	}
}

func TestSubscriptionGetByID(t *testing.T) {
	ss, as := setupSubscriptionTestDB(t)

	a, _ := as.Create("alice@example.com")
	created, _ := ss.Create(a.ID, "cloud")

	sub, err := ss.GetByID(created.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if sub == nil {
		t.Fatal("expected subscription, got nil")
	}
	if sub.ID != created.ID {
		t.Errorf("id = %d, want %d", sub.ID, created.ID)
	}
}

func TestSubscriptionGetByIDNotFound(t *testing.T) {
	ss, _ := setupSubscriptionTestDB(t)

	sub, err := ss.GetByID(999)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if sub != nil {
		t.Error("expected nil for nonexistent id")
	}
}

func TestSubscriptionGetByAccountID(t *testing.T) {
	ss, as := setupSubscriptionTestDB(t)

	a, _ := as.Create("alice@example.com")
	created, _ := ss.Create(a.ID, "cloud")

	sub, err := ss.GetByAccountID(a.ID)
	if err != nil {
		t.Fatalf("get by account id: %v", err)
	}
	if sub == nil {
		t.Fatal("expected subscription, got nil")
	}
	if sub.ID != created.ID {
		t.Errorf("id = %d, want %d", sub.ID, created.ID)
	}
}

func TestSubscriptionGetByStripeID(t *testing.T) {
	ss, as := setupSubscriptionTestDB(t)

	a, _ := as.Create("alice@example.com")
	created, _ := ss.Create(a.ID, "cloud")
	ss.UpdateStripeID(created.ID, "sub_123")

	sub, err := ss.GetByStripeID("sub_123")
	if err != nil {
		t.Fatalf("get by stripe id: %v", err)
	}
	if sub == nil {
		t.Fatal("expected subscription, got nil")
	}
	if sub.ID != created.ID {
		t.Errorf("id = %d, want %d", sub.ID, created.ID)
	}
}

func TestSubscriptionUpdateStatus(t *testing.T) {
	ss, as := setupSubscriptionTestDB(t)

	a, _ := as.Create("alice@example.com")
	created, _ := ss.Create(a.ID, "cloud")

	if err := ss.UpdateStatus(created.ID, "past_due"); err != nil {
		t.Fatalf("update status: %v", err)
	}

	sub, _ := ss.GetByID(created.ID)
	if sub.Status != "past_due" {
		t.Errorf("status = %q, want %q", sub.Status, "past_due")
	}
}

func TestSubscriptionSetCancelAtPeriodEnd(t *testing.T) {
	ss, as := setupSubscriptionTestDB(t)

	a, _ := as.Create("alice@example.com")
	created, _ := ss.Create(a.ID, "cloud")

	if err := ss.SetCancelAtPeriodEnd(created.ID, true); err != nil {
		t.Fatalf("set cancel: %v", err)
	}

	sub, _ := ss.GetByID(created.ID)
	if !sub.CancelAtPeriodEnd {
		t.Error("expected cancel_at_period_end = true")
	}
}

func TestSubscriptionDelete(t *testing.T) {
	ss, as := setupSubscriptionTestDB(t)

	a, _ := as.Create("alice@example.com")
	created, _ := ss.Create(a.ID, "cloud")

	if err := ss.Delete(created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	sub, err := ss.GetByID(created.ID)
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if sub != nil {
		t.Error("expected nil after delete")
	}
}
