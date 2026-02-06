package store

import (
	"testing"

	"github.com/dukerupert/gamwich/internal/database"
)

func setupHouseholdTestDB(t *testing.T) (*HouseholdStore, *UserStore) {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewHouseholdStore(db), NewUserStore(db)
}

func TestHouseholdCreate(t *testing.T) {
	hs, _ := setupHouseholdTestDB(t)

	h, err := hs.Create("Test Household")
	if err != nil {
		t.Fatalf("create household: %v", err)
	}
	if h.Name != "Test Household" {
		t.Errorf("name = %q, want %q", h.Name, "Test Household")
	}
	if h.ID == 0 {
		t.Error("expected non-zero ID")
	}
}

func TestHouseholdGetByID(t *testing.T) {
	hs, _ := setupHouseholdTestDB(t)

	created, err := hs.Create("Test Household")
	if err != nil {
		t.Fatalf("create household: %v", err)
	}

	h, err := hs.GetByID(created.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if h.Name != "Test Household" {
		t.Errorf("name = %q, want %q", h.Name, "Test Household")
	}
}

func TestHouseholdGetByIDNotFound(t *testing.T) {
	hs, _ := setupHouseholdTestDB(t)

	h, err := hs.GetByID(999)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if h != nil {
		t.Error("expected nil for nonexistent household")
	}
}

func TestHouseholdUpdate(t *testing.T) {
	hs, _ := setupHouseholdTestDB(t)

	created, err := hs.Create("Old Name")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := hs.Update(created.ID, "New Name")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("name = %q, want %q", updated.Name, "New Name")
	}
}

func TestHouseholdDelete(t *testing.T) {
	hs, _ := setupHouseholdTestDB(t)

	created, err := hs.Create("To Delete")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := hs.Delete(created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	h, err := hs.GetByID(created.ID)
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if h != nil {
		t.Error("expected nil after delete")
	}
}

func TestHouseholdAddMember(t *testing.T) {
	hs, us := setupHouseholdTestDB(t)

	h, err := hs.Create("Test Household")
	if err != nil {
		t.Fatalf("create household: %v", err)
	}
	u, err := us.Create("alice@example.com", "Alice")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	m, err := hs.AddMember(h.ID, u.ID, "admin")
	if err != nil {
		t.Fatalf("add member: %v", err)
	}
	if m.Role != "admin" {
		t.Errorf("role = %q, want %q", m.Role, "admin")
	}
	if m.HouseholdID != h.ID {
		t.Errorf("household_id = %d, want %d", m.HouseholdID, h.ID)
	}
	if m.UserID != u.ID {
		t.Errorf("user_id = %d, want %d", m.UserID, u.ID)
	}
}

func TestHouseholdAddMemberDuplicate(t *testing.T) {
	hs, us := setupHouseholdTestDB(t)

	h, _ := hs.Create("Test Household")
	u, _ := us.Create("alice@example.com", "Alice")

	if _, err := hs.AddMember(h.ID, u.ID, "admin"); err != nil {
		t.Fatalf("add member: %v", err)
	}
	if _, err := hs.AddMember(h.ID, u.ID, "member"); err == nil {
		t.Fatal("expected error for duplicate membership, got nil")
	}
}

func TestHouseholdRemoveMember(t *testing.T) {
	hs, us := setupHouseholdTestDB(t)

	h, _ := hs.Create("Test Household")
	u, _ := us.Create("alice@example.com", "Alice")
	hs.AddMember(h.ID, u.ID, "admin")

	if err := hs.RemoveMember(h.ID, u.ID); err != nil {
		t.Fatalf("remove member: %v", err)
	}

	m, err := hs.GetMember(h.ID, u.ID)
	if err != nil {
		t.Fatalf("get member after remove: %v", err)
	}
	if m != nil {
		t.Error("expected nil after remove")
	}
}

func TestHouseholdListMembers(t *testing.T) {
	hs, us := setupHouseholdTestDB(t)

	h, _ := hs.Create("Test Household")
	u1, _ := us.Create("alice@example.com", "Alice")
	u2, _ := us.Create("bob@example.com", "Bob")
	hs.AddMember(h.ID, u1.ID, "admin")
	hs.AddMember(h.ID, u2.ID, "member")

	members, err := hs.ListMembers(h.ID)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
}

func TestHouseholdListHouseholdsForUser(t *testing.T) {
	hs, us := setupHouseholdTestDB(t)

	h1, _ := hs.Create("Household A")
	h2, _ := hs.Create("Household B")
	u, _ := us.Create("alice@example.com", "Alice")
	hs.AddMember(h1.ID, u.ID, "admin")
	hs.AddMember(h2.ID, u.ID, "member")

	households, err := hs.ListHouseholdsForUser(u.ID)
	if err != nil {
		t.Fatalf("list households for user: %v", err)
	}
	if len(households) != 2 {
		t.Fatalf("expected 2 households, got %d", len(households))
	}
}

func TestHouseholdUpdateMemberRole(t *testing.T) {
	hs, us := setupHouseholdTestDB(t)

	h, _ := hs.Create("Test Household")
	u, _ := us.Create("alice@example.com", "Alice")
	hs.AddMember(h.ID, u.ID, "member")

	m, err := hs.UpdateMemberRole(h.ID, u.ID, "admin")
	if err != nil {
		t.Fatalf("update member role: %v", err)
	}
	if m.Role != "admin" {
		t.Errorf("role = %q, want %q", m.Role, "admin")
	}
}

func TestHouseholdSeedDefaults(t *testing.T) {
	hs, _ := setupHouseholdTestDB(t)

	h, err := hs.Create("New Household")
	if err != nil {
		t.Fatalf("create household: %v", err)
	}

	if err := hs.SeedDefaults(h.ID); err != nil {
		t.Fatalf("seed defaults: %v", err)
	}

	// Verify chore areas were created
	var areaCount int
	hs.db.QueryRow(`SELECT COUNT(*) FROM chore_areas WHERE household_id = ?`, h.ID).Scan(&areaCount)
	if areaCount != 5 {
		t.Errorf("chore areas = %d, want 5", areaCount)
	}

	// Verify grocery categories were created
	var catCount int
	hs.db.QueryRow(`SELECT COUNT(*) FROM grocery_categories WHERE household_id = ?`, h.ID).Scan(&catCount)
	if catCount != 11 {
		t.Errorf("grocery categories = %d, want 11", catCount)
	}

	// Verify grocery list was created
	var listCount int
	hs.db.QueryRow(`SELECT COUNT(*) FROM grocery_lists WHERE household_id = ?`, h.ID).Scan(&listCount)
	if listCount != 1 {
		t.Errorf("grocery lists = %d, want 1", listCount)
	}

	// Verify settings were created
	var settingsCount int
	hs.db.QueryRow(`SELECT COUNT(*) FROM settings WHERE household_id = ?`, h.ID).Scan(&settingsCount)
	if settingsCount != 13 {
		t.Errorf("settings = %d, want 13", settingsCount)
	}
}

func TestHouseholdDefaultSeed(t *testing.T) {
	hs, _ := setupHouseholdTestDB(t)

	// The migration seeds a default "My Household" with ID=1
	h, err := hs.GetByID(1)
	if err != nil {
		t.Fatalf("get default household: %v", err)
	}
	if h == nil {
		t.Fatal("expected default household with ID=1")
	}
	if h.Name != "My Household" {
		t.Errorf("name = %q, want %q", h.Name, "My Household")
	}
}
