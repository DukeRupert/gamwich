package store

import (
	"testing"
	"time"

	"github.com/dukerupert/gamwich/internal/database"
)

func setupChoreTestDB(t *testing.T) (*ChoreStore, *FamilyMemberStore) {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewChoreStore(db), NewFamilyMemberStore(db)
}

func TestAreaSeedData(t *testing.T) {
	cs, _ := setupChoreTestDB(t)

	areas, err := cs.ListAreas()
	if err != nil {
		t.Fatalf("list areas: %v", err)
	}
	if len(areas) != 5 {
		t.Fatalf("expected 5 seed areas, got %d", len(areas))
	}

	expected := []string{"Kitchen", "Bathroom", "Bedroom", "Yard", "General"}
	for i, name := range expected {
		if areas[i].Name != name {
			t.Errorf("area[%d].Name = %q, want %q", i, areas[i].Name, name)
		}
	}
}

func TestAreaCRUD(t *testing.T) {
	cs, _ := setupChoreTestDB(t)

	// Create
	area, err := cs.CreateArea("Garage", 6)
	if err != nil {
		t.Fatalf("create area: %v", err)
	}
	if area.Name != "Garage" {
		t.Errorf("name = %q, want %q", area.Name, "Garage")
	}
	if area.SortOrder != 6 {
		t.Errorf("sort_order = %d, want 6", area.SortOrder)
	}

	// Get
	got, err := cs.GetAreaByID(area.ID)
	if err != nil {
		t.Fatalf("get area: %v", err)
	}
	if got.Name != "Garage" {
		t.Errorf("got name = %q, want %q", got.Name, "Garage")
	}

	// Update
	updated, err := cs.UpdateArea(area.ID, "Garage/Workshop", 7)
	if err != nil {
		t.Fatalf("update area: %v", err)
	}
	if updated.Name != "Garage/Workshop" {
		t.Errorf("updated name = %q, want %q", updated.Name, "Garage/Workshop")
	}

	// Delete
	if err := cs.DeleteArea(area.ID); err != nil {
		t.Fatalf("delete area: %v", err)
	}
	got, err = cs.GetAreaByID(area.ID)
	if err != nil {
		t.Fatalf("get deleted area: %v", err)
	}
	if got != nil {
		t.Error("expected nil for deleted area")
	}
}

func TestAreaGetByIDNotFound(t *testing.T) {
	cs, _ := setupChoreTestDB(t)

	got, err := cs.GetAreaByID(9999)
	if err != nil {
		t.Fatalf("get area: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent area")
	}
}

func TestChoreCRUD(t *testing.T) {
	cs, _ := setupChoreTestDB(t)

	areas, _ := cs.ListAreas()
	kitchenID := areas[0].ID

	// Create
	chore, err := cs.Create("Wash dishes", "Clean all dishes", &kitchenID, 5, "FREQ=DAILY", nil)
	if err != nil {
		t.Fatalf("create chore: %v", err)
	}
	if chore.Title != "Wash dishes" {
		t.Errorf("title = %q, want %q", chore.Title, "Wash dishes")
	}
	if chore.Points != 5 {
		t.Errorf("points = %d, want 5", chore.Points)
	}
	if chore.AreaID == nil || *chore.AreaID != kitchenID {
		t.Errorf("area_id = %v, want %d", chore.AreaID, kitchenID)
	}
	if chore.RecurrenceRule != "FREQ=DAILY" {
		t.Errorf("recurrence_rule = %q, want %q", chore.RecurrenceRule, "FREQ=DAILY")
	}
	if chore.AssignedTo != nil {
		t.Errorf("assigned_to should be nil, got %v", *chore.AssignedTo)
	}

	// GetByID
	got, err := cs.GetByID(chore.ID)
	if err != nil {
		t.Fatalf("get chore: %v", err)
	}
	if got.Title != "Wash dishes" {
		t.Errorf("got title = %q, want %q", got.Title, "Wash dishes")
	}

	// Update
	updated, err := cs.Update(chore.ID, "Wash all dishes", "Pots and pans too", &kitchenID, 10, "FREQ=DAILY", nil)
	if err != nil {
		t.Fatalf("update chore: %v", err)
	}
	if updated.Title != "Wash all dishes" {
		t.Errorf("updated title = %q, want %q", updated.Title, "Wash all dishes")
	}
	if updated.Points != 10 {
		t.Errorf("updated points = %d, want 10", updated.Points)
	}

	// List
	chores, err := cs.List()
	if err != nil {
		t.Fatalf("list chores: %v", err)
	}
	if len(chores) != 1 {
		t.Fatalf("expected 1 chore, got %d", len(chores))
	}

	// Delete
	if err := cs.Delete(chore.ID); err != nil {
		t.Fatalf("delete chore: %v", err)
	}
	got, err = cs.GetByID(chore.ID)
	if err != nil {
		t.Fatalf("get deleted chore: %v", err)
	}
	if got != nil {
		t.Error("expected nil for deleted chore")
	}
}

func TestChoreGetByIDNotFound(t *testing.T) {
	cs, _ := setupChoreTestDB(t)

	got, err := cs.GetByID(9999)
	if err != nil {
		t.Fatalf("get chore: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent chore")
	}
}

func TestChoreListByAssignee(t *testing.T) {
	cs, ms := setupChoreTestDB(t)

	member, err := ms.Create("Alice", "#FF0000", "A")
	if err != nil {
		t.Fatalf("create member: %v", err)
	}

	cs.Create("Chore A", "", nil, 0, "", &member.ID)
	cs.Create("Chore B", "", nil, 0, "", nil)

	chores, err := cs.ListByAssignee(member.ID)
	if err != nil {
		t.Fatalf("list by assignee: %v", err)
	}
	if len(chores) != 1 {
		t.Fatalf("expected 1 chore, got %d", len(chores))
	}
	if chores[0].Title != "Chore A" {
		t.Errorf("title = %q, want %q", chores[0].Title, "Chore A")
	}
}

func TestChoreListByArea(t *testing.T) {
	cs, _ := setupChoreTestDB(t)

	areas, _ := cs.ListAreas()
	kitchenID := areas[0].ID
	bathroomID := areas[1].ID

	cs.Create("Kitchen chore", "", &kitchenID, 0, "", nil)
	cs.Create("Bathroom chore", "", &bathroomID, 0, "", nil)

	chores, err := cs.ListByArea(kitchenID)
	if err != nil {
		t.Fatalf("list by area: %v", err)
	}
	if len(chores) != 1 {
		t.Fatalf("expected 1 chore, got %d", len(chores))
	}
	if chores[0].Title != "Kitchen chore" {
		t.Errorf("title = %q, want %q", chores[0].Title, "Kitchen chore")
	}
}

func TestDeleteChoreCascadesCompletions(t *testing.T) {
	cs, ms := setupChoreTestDB(t)

	member, _ := ms.Create("Bob", "#0000FF", "B")
	chore, _ := cs.Create("Sweep floor", "", nil, 5, "", nil)

	_, err := cs.CreateCompletion(chore.ID, &member.ID, 0)
	if err != nil {
		t.Fatalf("create completion: %v", err)
	}

	completions, _ := cs.ListCompletionsByChore(chore.ID)
	if len(completions) != 1 {
		t.Fatalf("expected 1 completion, got %d", len(completions))
	}

	// Delete chore should cascade
	if err := cs.Delete(chore.ID); err != nil {
		t.Fatalf("delete chore: %v", err)
	}

	completions, _ = cs.ListCompletionsByChore(chore.ID)
	if len(completions) != 0 {
		t.Errorf("expected 0 completions after cascade, got %d", len(completions))
	}
}

func TestDeleteMemberSetsNullOnChore(t *testing.T) {
	cs, ms := setupChoreTestDB(t)

	member, _ := ms.Create("Charlie", "#00FF00", "C")
	chore, _ := cs.Create("Mow lawn", "", nil, 10, "", &member.ID)

	if chore.AssignedTo == nil || *chore.AssignedTo != member.ID {
		t.Fatalf("assigned_to = %v, want %d", chore.AssignedTo, member.ID)
	}

	if err := ms.Delete(member.ID); err != nil {
		t.Fatalf("delete member: %v", err)
	}

	got, err := cs.GetByID(chore.ID)
	if err != nil {
		t.Fatalf("get chore: %v", err)
	}
	if got.AssignedTo != nil {
		t.Errorf("assigned_to should be nil after member delete, got %v", *got.AssignedTo)
	}
}

func TestDeleteMemberSetsNullOnCompletion(t *testing.T) {
	cs, ms := setupChoreTestDB(t)

	member, _ := ms.Create("Diana", "#FFFF00", "D")
	chore, _ := cs.Create("Vacuum", "", nil, 5, "", nil)
	comp, _ := cs.CreateCompletion(chore.ID, &member.ID, 0)

	if comp.CompletedBy == nil || *comp.CompletedBy != member.ID {
		t.Fatalf("completed_by = %v, want %d", comp.CompletedBy, member.ID)
	}

	if err := ms.Delete(member.ID); err != nil {
		t.Fatalf("delete member: %v", err)
	}

	completions, _ := cs.ListCompletionsByChore(chore.ID)
	if len(completions) != 1 {
		t.Fatalf("expected 1 completion, got %d", len(completions))
	}
	if completions[0].CompletedBy != nil {
		t.Errorf("completed_by should be nil after member delete, got %v", *completions[0].CompletedBy)
	}
}

func TestDeleteAreaSetsNullOnChore(t *testing.T) {
	cs, _ := setupChoreTestDB(t)

	area, _ := cs.CreateArea("Test Area", 10)
	chore, _ := cs.Create("Test chore", "", &area.ID, 0, "", nil)

	if chore.AreaID == nil || *chore.AreaID != area.ID {
		t.Fatalf("area_id = %v, want %d", chore.AreaID, area.ID)
	}

	if err := cs.DeleteArea(area.ID); err != nil {
		t.Fatalf("delete area: %v", err)
	}

	got, err := cs.GetByID(chore.ID)
	if err != nil {
		t.Fatalf("get chore: %v", err)
	}
	if got.AreaID != nil {
		t.Errorf("area_id should be nil after area delete, got %v", *got.AreaID)
	}
}

func TestCompletionCRUD(t *testing.T) {
	cs, ms := setupChoreTestDB(t)

	member, _ := ms.Create("Eve", "#FF00FF", "E")
	chore, _ := cs.Create("Take out trash", "", nil, 3, "", nil)

	// Create completion
	comp, err := cs.CreateCompletion(chore.ID, &member.ID, 0)
	if err != nil {
		t.Fatalf("create completion: %v", err)
	}
	if comp.ChoreID != chore.ID {
		t.Errorf("chore_id = %d, want %d", comp.ChoreID, chore.ID)
	}
	if comp.CompletedBy == nil || *comp.CompletedBy != member.ID {
		t.Errorf("completed_by = %v, want %d", comp.CompletedBy, member.ID)
	}

	// List by chore
	completions, err := cs.ListCompletionsByChore(chore.ID)
	if err != nil {
		t.Fatalf("list completions: %v", err)
	}
	if len(completions) != 1 {
		t.Fatalf("expected 1 completion, got %d", len(completions))
	}

	// Last completion
	last, err := cs.LastCompletionForChore(chore.ID)
	if err != nil {
		t.Fatalf("last completion: %v", err)
	}
	if last.ID != comp.ID {
		t.Errorf("last completion id = %d, want %d", last.ID, comp.ID)
	}

	// Delete completion
	if err := cs.DeleteCompletion(comp.ID); err != nil {
		t.Fatalf("delete completion: %v", err)
	}
	completions, _ = cs.ListCompletionsByChore(chore.ID)
	if len(completions) != 0 {
		t.Errorf("expected 0 completions after delete, got %d", len(completions))
	}
}

func TestLastCompletionNotFound(t *testing.T) {
	cs, _ := setupChoreTestDB(t)

	chore, _ := cs.Create("No completions", "", nil, 0, "", nil)

	last, err := cs.LastCompletionForChore(chore.ID)
	if err != nil {
		t.Fatalf("last completion: %v", err)
	}
	if last != nil {
		t.Error("expected nil for chore with no completions")
	}
}

func TestCompletionNilCompletedBy(t *testing.T) {
	cs, _ := setupChoreTestDB(t)

	chore, _ := cs.Create("Anonymous chore", "", nil, 0, "", nil)

	comp, err := cs.CreateCompletion(chore.ID, nil, 0)
	if err != nil {
		t.Fatalf("create completion: %v", err)
	}
	if comp.CompletedBy != nil {
		t.Errorf("completed_by should be nil, got %v", *comp.CompletedBy)
	}
}

func TestListCompletionsByDateRange(t *testing.T) {
	cs, _ := setupChoreTestDB(t)

	chore, _ := cs.Create("Date range chore", "", nil, 0, "", nil)

	// Create a completion (uses default datetime('now'))
	_, err := cs.CreateCompletion(chore.ID, nil, 0)
	if err != nil {
		t.Fatalf("create completion: %v", err)
	}

	// Query a range that includes now
	now := time.Now().UTC()
	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Hour)

	completions, err := cs.ListCompletionsByDateRange(start, end)
	if err != nil {
		t.Fatalf("list by range: %v", err)
	}
	if len(completions) != 1 {
		t.Fatalf("expected 1 completion in range, got %d", len(completions))
	}

	// Query a range that excludes now
	farFuture := now.Add(24 * time.Hour)
	completions, err = cs.ListCompletionsByDateRange(farFuture, farFuture.Add(time.Hour))
	if err != nil {
		t.Fatalf("list by range: %v", err)
	}
	if len(completions) != 0 {
		t.Errorf("expected 0 completions in future range, got %d", len(completions))
	}
}

func TestAreaSortOrder(t *testing.T) {
	cs, _ := setupChoreTestDB(t)

	areas, _ := cs.ListAreas()
	// Reverse the order
	ids := make([]int64, len(areas))
	for i, a := range areas {
		ids[len(areas)-1-i] = a.ID
	}

	if err := cs.UpdateAreaSortOrder(ids); err != nil {
		t.Fatalf("update sort order: %v", err)
	}

	areas, _ = cs.ListAreas()
	// First area should now be what was last
	if areas[0].Name != "General" {
		t.Errorf("first area after reorder = %q, want %q", areas[0].Name, "General")
	}
}

func TestCompletionPointsEarned(t *testing.T) {
	cs, ms := setupChoreTestDB(t)

	member, _ := ms.Create("Frank", "#00FFFF", "F")
	chore, _ := cs.Create("Big task", "", nil, 25, "", nil)

	comp, err := cs.CreateCompletion(chore.ID, &member.ID, 25)
	if err != nil {
		t.Fatalf("create completion with points: %v", err)
	}
	if comp.PointsEarned != 25 {
		t.Errorf("points_earned = %d, want 25", comp.PointsEarned)
	}

	// Verify via list
	completions, _ := cs.ListCompletionsByChore(chore.ID)
	if len(completions) != 1 {
		t.Fatalf("expected 1 completion, got %d", len(completions))
	}
	if completions[0].PointsEarned != 25 {
		t.Errorf("listed points_earned = %d, want 25", completions[0].PointsEarned)
	}
}
