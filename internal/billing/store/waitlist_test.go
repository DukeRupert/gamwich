package store

import (
	"testing"

	"github.com/dukerupert/gamwich/internal/billing/database"
)

func setupWaitlistTestDB(t *testing.T) *WaitlistStore {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewWaitlistStore(db)
}

func TestWaitlistCreate(t *testing.T) {
	s := setupWaitlistTestDB(t)

	if err := s.Create("alice@example.com", "hosted"); err != nil {
		t.Fatalf("create: %v", err)
	}

	entries, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len = %d, want 1", len(entries))
	}
	if entries[0].Email != "alice@example.com" {
		t.Errorf("email = %q, want %q", entries[0].Email, "alice@example.com")
	}
	if entries[0].Plan != "hosted" {
		t.Errorf("plan = %q, want %q", entries[0].Plan, "hosted")
	}
}

func TestWaitlistDuplicateIdempotent(t *testing.T) {
	s := setupWaitlistTestDB(t)

	if err := s.Create("alice@example.com", "hosted"); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if err := s.Create("alice@example.com", "hosted"); err != nil {
		t.Fatalf("duplicate create: %v", err)
	}

	count, err := s.Count()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1 (duplicate should be ignored)", count)
	}
}

func TestWaitlistDifferentPlans(t *testing.T) {
	s := setupWaitlistTestDB(t)

	if err := s.Create("alice@example.com", "hosted"); err != nil {
		t.Fatalf("create hosted: %v", err)
	}
	if err := s.Create("alice@example.com", "cloud"); err != nil {
		t.Fatalf("create cloud: %v", err)
	}

	count, err := s.Count()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2 (different plans should be separate)", count)
	}
}

func TestWaitlistList(t *testing.T) {
	s := setupWaitlistTestDB(t)

	s.Create("alice@example.com", "hosted")
	s.Create("bob@example.com", "hosted")
	s.Create("carol@example.com", "hosted")

	entries, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("len = %d, want 3", len(entries))
	}
	// Should be ordered by created_at ASC
	if entries[0].Email != "alice@example.com" {
		t.Errorf("first entry = %q, want alice", entries[0].Email)
	}
	if entries[2].Email != "carol@example.com" {
		t.Errorf("last entry = %q, want carol", entries[2].Email)
	}
}

func TestWaitlistCount(t *testing.T) {
	s := setupWaitlistTestDB(t)

	count, err := s.Count()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("empty count = %d, want 0", count)
	}

	s.Create("alice@example.com", "hosted")
	s.Create("bob@example.com", "hosted")

	count, err = s.Count()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}
