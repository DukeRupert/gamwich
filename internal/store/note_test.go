package store

import (
	"testing"
	"time"

	"github.com/dukerupert/gamwich/internal/database"
)

func setupNoteTestDB(t *testing.T) (*NoteStore, *FamilyMemberStore) {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewNoteStore(db), NewFamilyMemberStore(db)
}

func TestNoteCRUD(t *testing.T) {
	ns, ms := setupNoteTestDB(t)

	member, _ := ms.Create("Alice", "#FF0000", "A")

	// Create
	note, err := ns.Create("Test Note", "Some body text", &member.ID, false, "normal", nil)
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if note.Title != "Test Note" {
		t.Errorf("title = %q, want %q", note.Title, "Test Note")
	}
	if note.Body != "Some body text" {
		t.Errorf("body = %q, want %q", note.Body, "Some body text")
	}
	if note.AuthorID == nil || *note.AuthorID != member.ID {
		t.Errorf("author_id = %v, want %d", note.AuthorID, member.ID)
	}
	if note.Pinned {
		t.Error("expected not pinned")
	}
	if note.Priority != "normal" {
		t.Errorf("priority = %q, want %q", note.Priority, "normal")
	}
	if note.ExpiresAt != nil {
		t.Errorf("expires_at = %v, want nil", note.ExpiresAt)
	}

	// Get by ID
	got, err := ns.GetByID(note.ID)
	if err != nil {
		t.Fatalf("get note: %v", err)
	}
	if got == nil {
		t.Fatal("expected note, got nil")
	}
	if got.Title != "Test Note" {
		t.Errorf("title = %q, want %q", got.Title, "Test Note")
	}

	// Update
	updated, err := ns.Update(note.ID, "Updated Title", "Updated body", &member.ID, true, "urgent", nil)
	if err != nil {
		t.Fatalf("update note: %v", err)
	}
	if updated.Title != "Updated Title" {
		t.Errorf("title = %q, want %q", updated.Title, "Updated Title")
	}
	if updated.Body != "Updated body" {
		t.Errorf("body = %q, want %q", updated.Body, "Updated body")
	}
	if !updated.Pinned {
		t.Error("expected pinned")
	}
	if updated.Priority != "urgent" {
		t.Errorf("priority = %q, want %q", updated.Priority, "urgent")
	}

	// Delete
	if err := ns.Delete(note.ID); err != nil {
		t.Fatalf("delete note: %v", err)
	}
	got, err = ns.GetByID(note.ID)
	if err != nil {
		t.Fatalf("get deleted note: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestNoteNotFound(t *testing.T) {
	ns, _ := setupNoteTestDB(t)

	got, err := ns.GetByID(999)
	if err != nil {
		t.Fatalf("get note: %v", err)
	}
	if got != nil {
		t.Error("expected nil for non-existent note")
	}
}

func TestNoteListOrdering(t *testing.T) {
	ns, _ := setupNoteTestDB(t)

	// Create notes with different priorities and pinned status
	ns.Create("Fun unpinned", "", nil, false, "fun", nil)
	ns.Create("Normal unpinned", "", nil, false, "normal", nil)
	ns.Create("Urgent unpinned", "", nil, false, "urgent", nil)
	ns.Create("Normal pinned", "", nil, true, "normal", nil)
	ns.Create("Urgent pinned", "", nil, true, "urgent", nil)

	notes, err := ns.List()
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	if len(notes) != 5 {
		t.Fatalf("expected 5 notes, got %d", len(notes))
	}

	// Pinned first (urgent pinned, then normal pinned), then unpinned (urgent, normal, fun)
	expected := []string{"Urgent pinned", "Normal pinned", "Urgent unpinned", "Normal unpinned", "Fun unpinned"}
	for i, e := range expected {
		if notes[i].Title != e {
			t.Errorf("notes[%d].Title = %q, want %q", i, notes[i].Title, e)
		}
	}
}

func TestNoteExpiredExclusion(t *testing.T) {
	ns, _ := setupNoteTestDB(t)

	// Create a note that already expired
	pastTime := time.Now().Add(-1 * time.Hour).UTC()
	ns.Create("Expired note", "", nil, false, "normal", &pastTime)

	// Create a note that hasn't expired
	futureTime := time.Now().Add(24 * time.Hour).UTC()
	ns.Create("Future note", "", nil, false, "normal", &futureTime)

	// Create a note with no expiry
	ns.Create("No expiry", "", nil, false, "normal", nil)

	notes, err := ns.List()
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	// Expired note should be cleaned up by lazy delete
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
	for _, n := range notes {
		if n.Title == "Expired note" {
			t.Error("expired note should not be in list")
		}
	}
}

func TestNoteListPinned(t *testing.T) {
	ns, _ := setupNoteTestDB(t)

	ns.Create("Pinned 1", "", nil, true, "normal", nil)
	ns.Create("Not pinned", "", nil, false, "normal", nil)
	ns.Create("Pinned 2", "", nil, true, "urgent", nil)

	pinned, err := ns.ListPinned()
	if err != nil {
		t.Fatalf("list pinned: %v", err)
	}
	if len(pinned) != 2 {
		t.Fatalf("expected 2 pinned notes, got %d", len(pinned))
	}
	// Should be ordered: urgent first, then normal
	if pinned[0].Title != "Pinned 2" {
		t.Errorf("pinned[0].Title = %q, want %q", pinned[0].Title, "Pinned 2")
	}
	if pinned[1].Title != "Pinned 1" {
		t.Errorf("pinned[1].Title = %q, want %q", pinned[1].Title, "Pinned 1")
	}
}

func TestNoteListPinnedExcludesExpired(t *testing.T) {
	ns, _ := setupNoteTestDB(t)

	pastTime := time.Now().Add(-1 * time.Hour).UTC()
	ns.Create("Expired pinned", "", nil, true, "normal", &pastTime)
	ns.Create("Active pinned", "", nil, true, "normal", nil)

	pinned, err := ns.ListPinned()
	if err != nil {
		t.Fatalf("list pinned: %v", err)
	}
	if len(pinned) != 1 {
		t.Fatalf("expected 1 pinned note, got %d", len(pinned))
	}
	if pinned[0].Title != "Active pinned" {
		t.Errorf("title = %q, want %q", pinned[0].Title, "Active pinned")
	}
}

func TestNoteTogglePinned(t *testing.T) {
	ns, _ := setupNoteTestDB(t)

	note, _ := ns.Create("Test", "", nil, false, "normal", nil)
	if note.Pinned {
		t.Error("expected not pinned initially")
	}

	// Toggle to pinned
	toggled, err := ns.TogglePinned(note.ID)
	if err != nil {
		t.Fatalf("toggle pinned: %v", err)
	}
	if !toggled.Pinned {
		t.Error("expected pinned after toggle")
	}

	// Toggle back to unpinned
	toggled, err = ns.TogglePinned(note.ID)
	if err != nil {
		t.Fatalf("toggle pinned back: %v", err)
	}
	if toggled.Pinned {
		t.Error("expected unpinned after second toggle")
	}
}

func TestNoteTogglePinnedNotFound(t *testing.T) {
	ns, _ := setupNoteTestDB(t)

	got, err := ns.TogglePinned(999)
	if err != nil {
		t.Fatalf("toggle pinned: %v", err)
	}
	if got != nil {
		t.Error("expected nil for non-existent note")
	}
}

func TestNoteDeleteExpired(t *testing.T) {
	ns, _ := setupNoteTestDB(t)

	pastTime := time.Now().Add(-1 * time.Hour).UTC()
	ns.Create("Expired 1", "", nil, false, "normal", &pastTime)
	ns.Create("Expired 2", "", nil, false, "normal", &pastTime)
	ns.Create("Active", "", nil, false, "normal", nil)

	count, err := ns.DeleteExpired()
	if err != nil {
		t.Fatalf("delete expired: %v", err)
	}
	if count != 2 {
		t.Errorf("deleted %d, want 2", count)
	}

	// Verify only active note remains (bypass lazy delete by using GetByID)
	notes, err := ns.List()
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	if notes[0].Title != "Active" {
		t.Errorf("title = %q, want %q", notes[0].Title, "Active")
	}
}

func TestNoteFKSetNullOnDeleteAuthor(t *testing.T) {
	ns, ms := setupNoteTestDB(t)

	member, _ := ms.Create("Alice", "#FF0000", "A")
	note, _ := ns.Create("Test", "", &member.ID, false, "normal", nil)

	if note.AuthorID == nil || *note.AuthorID != member.ID {
		t.Fatalf("author_id should be %d", member.ID)
	}

	// Delete the member
	if err := ms.Delete(member.ID); err != nil {
		t.Fatalf("delete member: %v", err)
	}

	// Note should still exist with NULL author_id
	got, err := ns.GetByID(note.ID)
	if err != nil {
		t.Fatalf("get note after author delete: %v", err)
	}
	if got == nil {
		t.Fatal("expected note to still exist")
	}
	if got.AuthorID != nil {
		t.Errorf("author_id = %v, want nil after delete", got.AuthorID)
	}
}

func TestNoteNilAuthor(t *testing.T) {
	ns, _ := setupNoteTestDB(t)

	note, err := ns.Create("No Author", "body", nil, false, "normal", nil)
	if err != nil {
		t.Fatalf("create note with nil author: %v", err)
	}
	if note.AuthorID != nil {
		t.Errorf("author_id = %v, want nil", note.AuthorID)
	}
}

func TestNoteWithExpiry(t *testing.T) {
	ns, _ := setupNoteTestDB(t)

	futureTime := time.Now().Add(48 * time.Hour).UTC()
	note, err := ns.Create("Expiring note", "", nil, false, "normal", &futureTime)
	if err != nil {
		t.Fatalf("create note with expiry: %v", err)
	}
	if note.ExpiresAt == nil {
		t.Fatal("expected expires_at to be set")
	}
}
