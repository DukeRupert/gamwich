package store

import (
	"testing"
	"time"

	"github.com/dukerupert/gamwich/internal/database"
	"github.com/dukerupert/gamwich/internal/model"
)

func setupBackupTestDB(t *testing.T) (*BackupStore, int64) {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Get the household created by migrations
	var householdID int64
	err = db.QueryRow("SELECT id FROM households LIMIT 1").Scan(&householdID)
	if err != nil {
		// Create a household if none exist
		result, err := db.Exec("INSERT INTO households (name) VALUES ('Test')")
		if err != nil {
			t.Fatalf("create household: %v", err)
		}
		householdID, _ = result.LastInsertId()
	}

	return NewBackupStore(db), householdID
}

func TestBackupCreate(t *testing.T) {
	bs, hid := setupBackupTestDB(t)

	b, err := bs.Create(hid, "backup-2024.db.enc", "1/2024-01-01T00:00:00Z.db.enc")
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if b.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if b.HouseholdID != hid {
		t.Errorf("household_id = %d, want %d", b.HouseholdID, hid)
	}
	if b.Filename != "backup-2024.db.enc" {
		t.Errorf("filename = %q, want %q", b.Filename, "backup-2024.db.enc")
	}
	if b.Status != model.BackupStatusPending {
		t.Errorf("status = %q, want %q", b.Status, model.BackupStatusPending)
	}
}

func TestBackupUpdateStatus(t *testing.T) {
	bs, hid := setupBackupTestDB(t)

	b, _ := bs.Create(hid, "test.db.enc", "1/test.db.enc")

	err := bs.UpdateStatus(b.ID, model.BackupStatusUploading, "")
	if err != nil {
		t.Fatalf("update status: %v", err)
	}

	got, _ := bs.GetByID(b.ID, hid)
	if got.Status != model.BackupStatusUploading {
		t.Errorf("status = %q, want %q", got.Status, model.BackupStatusUploading)
	}

	// Update with error
	err = bs.UpdateStatus(b.ID, model.BackupStatusFailed, "upload failed")
	if err != nil {
		t.Fatalf("update status with error: %v", err)
	}
	got, _ = bs.GetByID(b.ID, hid)
	if got.Status != model.BackupStatusFailed {
		t.Errorf("status = %q, want %q", got.Status, model.BackupStatusFailed)
	}
	if got.ErrorMessage != "upload failed" {
		t.Errorf("error_message = %q, want %q", got.ErrorMessage, "upload failed")
	}
}

func TestBackupUpdateCompleted(t *testing.T) {
	bs, hid := setupBackupTestDB(t)

	b, _ := bs.Create(hid, "test.db.enc", "1/test.db.enc")

	err := bs.UpdateCompleted(b.ID, 1024*1024)
	if err != nil {
		t.Fatalf("update completed: %v", err)
	}

	got, _ := bs.GetByID(b.ID, hid)
	if got.Status != model.BackupStatusCompleted {
		t.Errorf("status = %q, want %q", got.Status, model.BackupStatusCompleted)
	}
	if got.SizeBytes != 1024*1024 {
		t.Errorf("size_bytes = %d, want %d", got.SizeBytes, 1024*1024)
	}
	if got.CompletedAt == nil {
		t.Error("expected completed_at to be set")
	}
}

func TestBackupListOrderAndLimit(t *testing.T) {
	bs, hid := setupBackupTestDB(t)

	bs.Create(hid, "first.db.enc", "1/first.db.enc")
	time.Sleep(10 * time.Millisecond)
	bs.Create(hid, "second.db.enc", "1/second.db.enc")
	time.Sleep(10 * time.Millisecond)
	bs.Create(hid, "third.db.enc", "1/third.db.enc")

	// List all
	all, err := bs.List(hid, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("len = %d, want 3", len(all))
	}
	// Should be newest first
	if all[0].Filename != "third.db.enc" {
		t.Errorf("first entry = %q, want %q", all[0].Filename, "third.db.enc")
	}

	// Limit
	limited, err := bs.List(hid, 2)
	if err != nil {
		t.Fatalf("list limited: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("len = %d, want 2", len(limited))
	}
}

func TestBackupDeleteOlderThan(t *testing.T) {
	bs, hid := setupBackupTestDB(t)

	bs.Create(hid, "old.db.enc", "1/old.db.enc")
	time.Sleep(50 * time.Millisecond)
	cutoff := time.Now().UTC()
	time.Sleep(50 * time.Millisecond)
	bs.Create(hid, "new.db.enc", "1/new.db.enc")

	keys, err := bs.DeleteOlderThan(hid, cutoff)
	if err != nil {
		t.Fatalf("delete older than: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("deleted keys = %d, want 1", len(keys))
	}
	if keys[0] != "1/old.db.enc" {
		t.Errorf("deleted key = %q, want %q", keys[0], "1/old.db.enc")
	}

	remaining, _ := bs.List(hid, 10)
	if len(remaining) != 1 {
		t.Fatalf("remaining = %d, want 1", len(remaining))
	}
	if remaining[0].Filename != "new.db.enc" {
		t.Errorf("remaining = %q, want %q", remaining[0].Filename, "new.db.enc")
	}
}

func TestBackupLatestCompleted(t *testing.T) {
	bs, hid := setupBackupTestDB(t)

	b1, _ := bs.Create(hid, "first.db.enc", "1/first.db.enc")
	bs.UpdateCompleted(b1.ID, 100)
	time.Sleep(10 * time.Millisecond)
	b2, _ := bs.Create(hid, "second.db.enc", "1/second.db.enc")
	bs.UpdateCompleted(b2.ID, 200)

	// Also create a failed one that shouldn't be returned
	b3, _ := bs.Create(hid, "failed.db.enc", "1/failed.db.enc")
	bs.UpdateStatus(b3.ID, model.BackupStatusFailed, "error")

	latest, err := bs.LatestCompleted(hid)
	if err != nil {
		t.Fatalf("latest completed: %v", err)
	}
	if latest == nil {
		t.Fatal("expected latest, got nil")
	}
	if latest.Filename != "second.db.enc" {
		t.Errorf("filename = %q, want %q", latest.Filename, "second.db.enc")
	}
}

func TestBackupTotalSize(t *testing.T) {
	bs, hid := setupBackupTestDB(t)

	b1, _ := bs.Create(hid, "a.db.enc", "1/a.db.enc")
	bs.UpdateCompleted(b1.ID, 1000)
	b2, _ := bs.Create(hid, "b.db.enc", "1/b.db.enc")
	bs.UpdateCompleted(b2.ID, 2500)

	// Failed backup should not count
	bs.Create(hid, "c.db.enc", "1/c.db.enc")

	total, err := bs.TotalSizeByHousehold(hid)
	if err != nil {
		t.Fatalf("total size: %v", err)
	}
	if total != 3500 {
		t.Errorf("total = %d, want 3500", total)
	}
}

func TestBackupHouseholdIsolation(t *testing.T) {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Create two households
	r1, _ := db.Exec("INSERT INTO households (name) VALUES ('House A')")
	hid1, _ := r1.LastInsertId()
	r2, _ := db.Exec("INSERT INTO households (name) VALUES ('House B')")
	hid2, _ := r2.LastInsertId()

	bs := NewBackupStore(db)

	bs.Create(hid1, "a.db.enc", "a/a.db.enc")
	bs.Create(hid1, "b.db.enc", "a/b.db.enc")
	bs.Create(hid2, "c.db.enc", "b/c.db.enc")

	list1, _ := bs.List(hid1, 10)
	list2, _ := bs.List(hid2, 10)

	if len(list1) != 2 {
		t.Errorf("household 1 backups = %d, want 2", len(list1))
	}
	if len(list2) != 1 {
		t.Errorf("household 2 backups = %d, want 1", len(list2))
	}

	// GetByID should not cross households
	b1 := list1[0]
	got, _ := bs.GetByID(b1.ID, hid2)
	if got != nil {
		t.Error("expected nil when querying wrong household")
	}
}
