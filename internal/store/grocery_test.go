package store

import (
	"testing"

	"github.com/dukerupert/gamwich/internal/database"
)

func setupGroceryTestDB(t *testing.T) (*GroceryStore, *FamilyMemberStore) {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewGroceryStore(db), NewFamilyMemberStore(db)
}

func TestCategorySeedData(t *testing.T) {
	gs, _ := setupGroceryTestDB(t)

	categories, err := gs.ListCategories()
	if err != nil {
		t.Fatalf("list categories: %v", err)
	}
	if len(categories) != 11 {
		t.Fatalf("expected 11 seed categories, got %d", len(categories))
	}

	expected := []string{"Produce", "Dairy", "Meat & Seafood", "Bakery", "Pantry", "Frozen", "Beverages", "Snacks", "Household", "Personal Care", "Other"}
	for i, name := range expected {
		if categories[i].Name != name {
			t.Errorf("category[%d].Name = %q, want %q", i, categories[i].Name, name)
		}
	}
}

func TestDefaultListSeedData(t *testing.T) {
	gs, _ := setupGroceryTestDB(t)

	list, err := gs.GetDefaultList()
	if err != nil {
		t.Fatalf("get default list: %v", err)
	}
	if list == nil {
		t.Fatal("expected default list, got nil")
	}
	if list.Name != "Grocery" {
		t.Errorf("default list name = %q, want %q", list.Name, "Grocery")
	}
}

func TestItemCRUD(t *testing.T) {
	gs, ms := setupGroceryTestDB(t)

	list, _ := gs.GetDefaultList()
	member, _ := ms.Create("Alice", "#FF0000", "A")

	// Create
	item, err := gs.CreateItem(list.ID, "Milk", "1", "gallon", "whole milk", "Dairy", &member.ID)
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	if item.Name != "Milk" {
		t.Errorf("name = %q, want %q", item.Name, "Milk")
	}
	if item.Quantity != "1" {
		t.Errorf("quantity = %q, want %q", item.Quantity, "1")
	}
	if item.Unit != "gallon" {
		t.Errorf("unit = %q, want %q", item.Unit, "gallon")
	}
	if item.Category != "Dairy" {
		t.Errorf("category = %q, want %q", item.Category, "Dairy")
	}
	if item.Checked {
		t.Error("expected unchecked")
	}
	if item.AddedBy == nil || *item.AddedBy != member.ID {
		t.Errorf("added_by = %v, want %d", item.AddedBy, member.ID)
	}

	// GetByID
	got, err := gs.GetItemByID(item.ID)
	if err != nil {
		t.Fatalf("get item: %v", err)
	}
	if got.Name != "Milk" {
		t.Errorf("got name = %q, want %q", got.Name, "Milk")
	}

	// Update
	updated, err := gs.UpdateItem(item.ID, "Whole Milk", "2", "gallons", "organic", "Dairy")
	if err != nil {
		t.Fatalf("update item: %v", err)
	}
	if updated.Name != "Whole Milk" {
		t.Errorf("updated name = %q, want %q", updated.Name, "Whole Milk")
	}
	if updated.Quantity != "2" {
		t.Errorf("updated quantity = %q, want %q", updated.Quantity, "2")
	}

	// List
	items, err := gs.ListItemsByList(list.ID)
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	// Delete
	if err := gs.DeleteItem(item.ID); err != nil {
		t.Fatalf("delete item: %v", err)
	}
	got, err = gs.GetItemByID(item.ID)
	if err != nil {
		t.Fatalf("get deleted item: %v", err)
	}
	if got != nil {
		t.Error("expected nil for deleted item")
	}
}

func TestItemGetByIDNotFound(t *testing.T) {
	gs, _ := setupGroceryTestDB(t)

	got, err := gs.GetItemByID(9999)
	if err != nil {
		t.Fatalf("get item: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent item")
	}
}

func TestItemListOrdering(t *testing.T) {
	gs, _ := setupGroceryTestDB(t)

	list, _ := gs.GetDefaultList()

	// Create items in different categories
	gs.CreateItem(list.ID, "Chicken", "", "", "", "Meat & Seafood", nil)
	gs.CreateItem(list.ID, "Apples", "", "", "", "Produce", nil) // earlier sort
	gs.CreateItem(list.ID, "Bread", "", "", "", "Bakery", nil)

	items, err := gs.ListItemsByList(list.ID)
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// Should be ordered by category ASC (Bakery < Meat & Seafood < Produce)
	if items[0].Name != "Bread" {
		t.Errorf("items[0].Name = %q, want %q (Bakery)", items[0].Name, "Bread")
	}
	if items[1].Name != "Chicken" {
		t.Errorf("items[1].Name = %q, want %q (Meat & Seafood)", items[1].Name, "Chicken")
	}
	if items[2].Name != "Apples" {
		t.Errorf("items[2].Name = %q, want %q (Produce)", items[2].Name, "Apples")
	}
}

func TestItemCheckedAfterUnchecked(t *testing.T) {
	gs, _ := setupGroceryTestDB(t)

	list, _ := gs.GetDefaultList()

	item1, _ := gs.CreateItem(list.ID, "Milk", "", "", "", "Dairy", nil)
	gs.CreateItem(list.ID, "Bread", "", "", "", "Bakery", nil)

	// Check item1
	gs.ToggleChecked(item1.ID, nil)

	items, err := gs.ListItemsByList(list.ID)
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// Unchecked items should come first
	if items[0].Checked {
		t.Error("first item should be unchecked")
	}
	if !items[1].Checked {
		t.Error("second item should be checked")
	}
}

func TestToggleChecked(t *testing.T) {
	gs, ms := setupGroceryTestDB(t)

	list, _ := gs.GetDefaultList()
	member, _ := ms.Create("Bob", "#0000FF", "B")

	item, _ := gs.CreateItem(list.ID, "Eggs", "", "", "", "Dairy", nil)

	// Check
	checked, err := gs.ToggleChecked(item.ID, &member.ID)
	if err != nil {
		t.Fatalf("toggle check: %v", err)
	}
	if !checked.Checked {
		t.Error("expected checked = true")
	}
	if checked.CheckedBy == nil || *checked.CheckedBy != member.ID {
		t.Errorf("checked_by = %v, want %d", checked.CheckedBy, member.ID)
	}
	if checked.CheckedAt == nil {
		t.Error("checked_at should not be nil")
	}

	// Uncheck
	unchecked, err := gs.ToggleChecked(item.ID, &member.ID)
	if err != nil {
		t.Fatalf("toggle uncheck: %v", err)
	}
	if unchecked.Checked {
		t.Error("expected checked = false")
	}
	if unchecked.CheckedBy != nil {
		t.Errorf("checked_by should be nil, got %v", *unchecked.CheckedBy)
	}
	if unchecked.CheckedAt != nil {
		t.Error("checked_at should be nil")
	}
}

func TestToggleCheckedNotFound(t *testing.T) {
	gs, _ := setupGroceryTestDB(t)

	got, err := gs.ToggleChecked(9999, nil)
	if err != nil {
		t.Fatalf("toggle checked: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent item")
	}
}

func TestClearChecked(t *testing.T) {
	gs, _ := setupGroceryTestDB(t)

	list, _ := gs.GetDefaultList()

	item1, _ := gs.CreateItem(list.ID, "Milk", "", "", "", "Dairy", nil)
	item2, _ := gs.CreateItem(list.ID, "Bread", "", "", "", "Bakery", nil)
	gs.CreateItem(list.ID, "Eggs", "", "", "", "Dairy", nil) // unchecked

	gs.ToggleChecked(item1.ID, nil)
	gs.ToggleChecked(item2.ID, nil)

	count, err := gs.ClearChecked(list.ID)
	if err != nil {
		t.Fatalf("clear checked: %v", err)
	}
	if count != 2 {
		t.Errorf("cleared count = %d, want 2", count)
	}

	items, _ := gs.ListItemsByList(list.ID)
	if len(items) != 1 {
		t.Fatalf("expected 1 remaining item, got %d", len(items))
	}
	if items[0].Name != "Eggs" {
		t.Errorf("remaining item = %q, want %q", items[0].Name, "Eggs")
	}
}

func TestCountUnchecked(t *testing.T) {
	gs, _ := setupGroceryTestDB(t)

	list, _ := gs.GetDefaultList()

	item1, _ := gs.CreateItem(list.ID, "Milk", "", "", "", "Dairy", nil)
	gs.CreateItem(list.ID, "Bread", "", "", "", "Bakery", nil)
	gs.CreateItem(list.ID, "Eggs", "", "", "", "Dairy", nil)

	count, err := gs.CountUnchecked(list.ID)
	if err != nil {
		t.Fatalf("count unchecked: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}

	gs.ToggleChecked(item1.ID, nil)

	count, err = gs.CountUnchecked(list.ID)
	if err != nil {
		t.Fatalf("count unchecked: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestDeleteListCascadesItems(t *testing.T) {
	gs, _ := setupGroceryTestDB(t)

	list, _ := gs.GetDefaultList()
	gs.CreateItem(list.ID, "Milk", "", "", "", "Dairy", nil)
	gs.CreateItem(list.ID, "Bread", "", "", "", "Bakery", nil)

	// Delete list directly via SQL
	_, err := gs.db.Exec(`DELETE FROM grocery_lists WHERE id = ?`, list.ID)
	if err != nil {
		t.Fatalf("delete list: %v", err)
	}

	items, _ := gs.ListItemsByList(list.ID)
	if len(items) != 0 {
		t.Errorf("expected 0 items after cascade, got %d", len(items))
	}
}

func TestDeleteMemberSetsNullOnItems(t *testing.T) {
	gs, ms := setupGroceryTestDB(t)

	list, _ := gs.GetDefaultList()
	member, _ := ms.Create("Charlie", "#00FF00", "C")

	item, _ := gs.CreateItem(list.ID, "Milk", "", "", "", "Dairy", &member.ID)

	// Check the item with the member
	gs.ToggleChecked(item.ID, &member.ID)

	// Verify both FKs are set
	got, _ := gs.GetItemByID(item.ID)
	if got.AddedBy == nil || *got.AddedBy != member.ID {
		t.Fatalf("added_by = %v, want %d", got.AddedBy, member.ID)
	}
	if got.CheckedBy == nil || *got.CheckedBy != member.ID {
		t.Fatalf("checked_by = %v, want %d", got.CheckedBy, member.ID)
	}

	// Delete member
	if err := ms.Delete(member.ID); err != nil {
		t.Fatalf("delete member: %v", err)
	}

	got, err := gs.GetItemByID(item.ID)
	if err != nil {
		t.Fatalf("get item: %v", err)
	}
	if got.AddedBy != nil {
		t.Errorf("added_by should be nil after member delete, got %v", *got.AddedBy)
	}
	if got.CheckedBy != nil {
		t.Errorf("checked_by should be nil after member delete, got %v", *got.CheckedBy)
	}
}

func TestCreateItemNilAddedBy(t *testing.T) {
	gs, _ := setupGroceryTestDB(t)

	list, _ := gs.GetDefaultList()

	item, err := gs.CreateItem(list.ID, "Bread", "", "", "", "Bakery", nil)
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	if item.AddedBy != nil {
		t.Errorf("added_by should be nil, got %v", *item.AddedBy)
	}
}
