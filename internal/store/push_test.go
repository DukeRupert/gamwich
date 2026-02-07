package store

import (
	"testing"
	"time"

	"github.com/dukerupert/gamwich/internal/database"
	"github.com/dukerupert/gamwich/internal/model"
)

func setupPushTestDB(t *testing.T) (*PushStore, int64, int64) {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Create a household
	var householdID int64
	err = db.QueryRow("SELECT id FROM households LIMIT 1").Scan(&householdID)
	if err != nil {
		result, err := db.Exec("INSERT INTO households (name) VALUES ('Test')")
		if err != nil {
			t.Fatalf("create household: %v", err)
		}
		householdID, _ = result.LastInsertId()
	}

	// Create a user
	result, err := db.Exec("INSERT INTO users (email) VALUES ('test@example.com')")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	userID, _ := result.LastInsertId()

	// Link user to household
	db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (?, ?, 'admin')", householdID, userID)

	return NewPushStore(db), householdID, userID
}

func TestCreateSubscription(t *testing.T) {
	ps, hid, uid := setupPushTestDB(t)

	sub, err := ps.CreateSubscription(uid, hid, "https://push.example.com/sub1", "p256dh_key1", "auth_key1", "Chrome Desktop")
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	if sub.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if sub.Endpoint != "https://push.example.com/sub1" {
		t.Errorf("endpoint = %q, want %q", sub.Endpoint, "https://push.example.com/sub1")
	}
	if sub.DeviceName != "Chrome Desktop" {
		t.Errorf("device_name = %q, want %q", sub.DeviceName, "Chrome Desktop")
	}
}

func TestCreateSubscriptionUpsert(t *testing.T) {
	ps, hid, uid := setupPushTestDB(t)

	sub1, _ := ps.CreateSubscription(uid, hid, "https://push.example.com/sub1", "key1", "auth1", "Device A")
	sub2, err := ps.CreateSubscription(uid, hid, "https://push.example.com/sub1", "key2", "auth2", "Device B")
	if err != nil {
		t.Fatalf("upsert subscription: %v", err)
	}

	// Should be same subscription, updated keys
	if sub2.ID != sub1.ID {
		t.Errorf("expected same ID on upsert, got %d != %d", sub2.ID, sub1.ID)
	}
	if sub2.P256dhKey != "key2" {
		t.Errorf("p256dh = %q, want %q", sub2.P256dhKey, "key2")
	}
}

func TestListByUser(t *testing.T) {
	ps, hid, uid := setupPushTestDB(t)

	ps.CreateSubscription(uid, hid, "https://push.example.com/1", "k1", "a1", "Device 1")
	ps.CreateSubscription(uid, hid, "https://push.example.com/2", "k2", "a2", "Device 2")

	subs, err := ps.ListByUser(uid, hid)
	if err != nil {
		t.Fatalf("list by user: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("len = %d, want 2", len(subs))
	}
}

func TestListByHousehold(t *testing.T) {
	ps, hid, uid := setupPushTestDB(t)

	ps.CreateSubscription(uid, hid, "https://push.example.com/1", "k1", "a1", "D1")

	subs, err := ps.ListByHousehold(hid)
	if err != nil {
		t.Fatalf("list by household: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("len = %d, want 1", len(subs))
	}
}

func TestDeleteSubscription(t *testing.T) {
	ps, hid, uid := setupPushTestDB(t)

	sub, _ := ps.CreateSubscription(uid, hid, "https://push.example.com/1", "k1", "a1", "D1")

	err := ps.DeleteSubscription(sub.ID, hid)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	subs, _ := ps.ListByUser(uid, hid)
	if len(subs) != 0 {
		t.Errorf("expected 0 subs after delete, got %d", len(subs))
	}
}

func TestDeleteByEndpoint(t *testing.T) {
	ps, hid, uid := setupPushTestDB(t)

	ps.CreateSubscription(uid, hid, "https://push.example.com/expired", "k1", "a1", "D1")

	err := ps.DeleteByEndpoint("https://push.example.com/expired")
	if err != nil {
		t.Fatalf("delete by endpoint: %v", err)
	}

	subs, _ := ps.ListByUser(uid, hid)
	if len(subs) != 0 {
		t.Errorf("expected 0 subs, got %d", len(subs))
	}
}

func TestListHouseholdIDs(t *testing.T) {
	ps, hid, uid := setupPushTestDB(t)

	ps.CreateSubscription(uid, hid, "https://push.example.com/1", "k1", "a1", "D1")

	ids, err := ps.ListHouseholdIDs()
	if err != nil {
		t.Fatalf("list household ids: %v", err)
	}
	if len(ids) != 1 || ids[0] != hid {
		t.Errorf("ids = %v, want [%d]", ids, hid)
	}
}

func TestPreferences(t *testing.T) {
	ps, hid, uid := setupPushTestDB(t)

	// Default: no prefs exist, IsPreferenceEnabled returns true
	enabled, err := ps.IsPreferenceEnabled(uid, hid, model.NotifTypeCalendarReminder)
	if err != nil {
		t.Fatalf("check default pref: %v", err)
	}
	if !enabled {
		t.Error("expected default enabled=true")
	}

	// Set a preference
	if err := ps.SetPreference(uid, hid, model.NotifTypeCalendarReminder, false); err != nil {
		t.Fatalf("set preference: %v", err)
	}

	enabled, err = ps.IsPreferenceEnabled(uid, hid, model.NotifTypeCalendarReminder)
	if err != nil {
		t.Fatalf("check disabled pref: %v", err)
	}
	if enabled {
		t.Error("expected enabled=false after setting")
	}

	// List preferences
	prefs, err := ps.GetPreferences(uid, hid)
	if err != nil {
		t.Fatalf("get preferences: %v", err)
	}
	if len(prefs) != 1 {
		t.Fatalf("prefs len = %d, want 1", len(prefs))
	}
	if prefs[0].NotificationType != model.NotifTypeCalendarReminder {
		t.Errorf("type = %q, want %q", prefs[0].NotificationType, model.NotifTypeCalendarReminder)
	}

	// Upsert: re-enable
	if err := ps.SetPreference(uid, hid, model.NotifTypeCalendarReminder, true); err != nil {
		t.Fatalf("upsert preference: %v", err)
	}
	enabled, _ = ps.IsPreferenceEnabled(uid, hid, model.NotifTypeCalendarReminder)
	if !enabled {
		t.Error("expected enabled=true after upsert")
	}
}

func TestSentNotificationDedup(t *testing.T) {
	ps, hid, _ := setupPushTestDB(t)

	// Not yet sent
	sent, err := ps.WasSent(hid, model.NotifTypeCalendarReminder, "event-42", 15)
	if err != nil {
		t.Fatalf("was sent: %v", err)
	}
	if sent {
		t.Error("expected not sent")
	}

	// Record sent
	if err := ps.RecordSent(hid, model.NotifTypeCalendarReminder, "event-42", 15); err != nil {
		t.Fatalf("record sent: %v", err)
	}

	// Now it's sent
	sent, _ = ps.WasSent(hid, model.NotifTypeCalendarReminder, "event-42", 15)
	if !sent {
		t.Error("expected sent after recording")
	}

	// Different lead time is separate
	sent, _ = ps.WasSent(hid, model.NotifTypeCalendarReminder, "event-42", 60)
	if sent {
		t.Error("expected not sent for different lead time")
	}

	// Duplicate insert is ignored (INSERT OR IGNORE)
	if err := ps.RecordSent(hid, model.NotifTypeCalendarReminder, "event-42", 15); err != nil {
		t.Fatalf("duplicate record sent should not error: %v", err)
	}
}

func TestCleanupSent(t *testing.T) {
	ps, hid, _ := setupPushTestDB(t)

	ps.RecordSent(hid, model.NotifTypeCalendarReminder, "old-event", 15)
	ps.RecordSent(hid, model.NotifTypeCalendarReminder, "new-event", 15)

	// Clean up everything older than 1 hour from now (should delete nothing)
	futureCleanup := time.Now().UTC().Add(-1 * time.Hour)
	if err := ps.CleanupSent(futureCleanup); err != nil {
		t.Fatalf("cleanup sent: %v", err)
	}
	sent, _ := ps.WasSent(hid, model.NotifTypeCalendarReminder, "old-event", 15)
	if !sent {
		t.Error("expected old notification to still exist (cutoff in past)")
	}

	// Clean up everything (cutoff in the future)
	if err := ps.CleanupSent(time.Now().UTC().Add(1 * time.Hour)); err != nil {
		t.Fatalf("cleanup sent: %v", err)
	}
	sent, _ = ps.WasSent(hid, model.NotifTypeCalendarReminder, "old-event", 15)
	if sent {
		t.Error("expected old notification to be cleaned up")
	}
	sent, _ = ps.WasSent(hid, model.NotifTypeCalendarReminder, "new-event", 15)
	if sent {
		t.Error("expected new notification to be cleaned up")
	}
}

func TestHouseholdIsolation(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	r1, _ := db.Exec("INSERT INTO households (name) VALUES ('House A')")
	hid1, _ := r1.LastInsertId()
	r2, _ := db.Exec("INSERT INTO households (name) VALUES ('House B')")
	hid2, _ := r2.LastInsertId()

	r3, _ := db.Exec("INSERT INTO users (email) VALUES ('user1@test.com')")
	uid1, _ := r3.LastInsertId()
	r4, _ := db.Exec("INSERT INTO users (email) VALUES ('user2@test.com')")
	uid2, _ := r4.LastInsertId()

	db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (?, ?, 'admin')", hid1, uid1)
	db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (?, ?, 'admin')", hid2, uid2)

	ps := NewPushStore(db)

	ps.CreateSubscription(uid1, hid1, "https://push.example.com/a", "k1", "a1", "D1")
	ps.CreateSubscription(uid2, hid2, "https://push.example.com/b", "k2", "a2", "D2")

	subs1, _ := ps.ListByHousehold(hid1)
	subs2, _ := ps.ListByHousehold(hid2)

	if len(subs1) != 1 {
		t.Errorf("household 1 subs = %d, want 1", len(subs1))
	}
	if len(subs2) != 1 {
		t.Errorf("household 2 subs = %d, want 1", len(subs2))
	}

	// Cannot delete across households
	err = ps.DeleteSubscription(subs1[0].ID, hid2)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	// Should still exist in hid1
	remaining, _ := ps.ListByHousehold(hid1)
	if len(remaining) != 1 {
		t.Errorf("sub should still exist, got %d", len(remaining))
	}
}
