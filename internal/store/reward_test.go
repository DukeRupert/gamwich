package store

import (
	"database/sql"
	"testing"

	"github.com/dukerupert/gamwich/internal/database"
)

func setupRewardTestDB(t *testing.T) (*RewardStore, *ChoreStore, *FamilyMemberStore) {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewRewardStore(db), NewChoreStore(db), NewFamilyMemberStore(db)
}

func TestRewardCRUD(t *testing.T) {
	rs, _, _ := setupRewardTestDB(t)

	// Create
	reward, err := rs.Create("Ice Cream Trip", "Go get ice cream!", 50, true)
	if err != nil {
		t.Fatalf("create reward: %v", err)
	}
	if reward.Title != "Ice Cream Trip" {
		t.Errorf("title = %q, want %q", reward.Title, "Ice Cream Trip")
	}
	if reward.Description != "Go get ice cream!" {
		t.Errorf("description = %q, want %q", reward.Description, "Go get ice cream!")
	}
	if reward.PointCost != 50 {
		t.Errorf("point_cost = %d, want 50", reward.PointCost)
	}
	if !reward.Active {
		t.Error("expected active")
	}

	// Get by ID
	got, err := rs.GetByID(reward.ID)
	if err != nil {
		t.Fatalf("get reward: %v", err)
	}
	if got == nil {
		t.Fatal("expected reward, got nil")
	}
	if got.Title != "Ice Cream Trip" {
		t.Errorf("title = %q, want %q", got.Title, "Ice Cream Trip")
	}

	// Update
	updated, err := rs.Update(reward.ID, "Movie Night", "Watch a movie", 100, true)
	if err != nil {
		t.Fatalf("update reward: %v", err)
	}
	if updated.Title != "Movie Night" {
		t.Errorf("title = %q, want %q", updated.Title, "Movie Night")
	}
	if updated.PointCost != 100 {
		t.Errorf("point_cost = %d, want 100", updated.PointCost)
	}

	// Delete
	if err := rs.Delete(reward.ID); err != nil {
		t.Fatalf("delete reward: %v", err)
	}
	got, err = rs.GetByID(reward.ID)
	if err != nil {
		t.Fatalf("get deleted reward: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestRewardNotFound(t *testing.T) {
	rs, _, _ := setupRewardTestDB(t)

	got, err := rs.GetByID(999)
	if err != nil {
		t.Fatalf("get reward: %v", err)
	}
	if got != nil {
		t.Error("expected nil for non-existent reward")
	}
}

func TestRewardListOrdering(t *testing.T) {
	rs, _, _ := setupRewardTestDB(t)

	rs.Create("Zebra Reward", "", 10, true)
	rs.Create("Alpha Reward", "", 20, true)
	rs.Create("Beta Inactive", "", 5, false)

	rewards, err := rs.List()
	if err != nil {
		t.Fatalf("list rewards: %v", err)
	}
	if len(rewards) != 3 {
		t.Fatalf("expected 3 rewards, got %d", len(rewards))
	}

	// Active first (alpha, zebra), then inactive (beta)
	if rewards[0].Title != "Alpha Reward" {
		t.Errorf("rewards[0].Title = %q, want %q", rewards[0].Title, "Alpha Reward")
	}
	if rewards[1].Title != "Zebra Reward" {
		t.Errorf("rewards[1].Title = %q, want %q", rewards[1].Title, "Zebra Reward")
	}
	if rewards[2].Title != "Beta Inactive" {
		t.Errorf("rewards[2].Title = %q, want %q", rewards[2].Title, "Beta Inactive")
	}
}

func TestRewardListActive(t *testing.T) {
	rs, _, _ := setupRewardTestDB(t)

	rs.Create("Active One", "", 10, true)
	rs.Create("Inactive", "", 20, false)
	rs.Create("Active Two", "", 30, true)

	active, err := rs.ListActive()
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 2 {
		t.Fatalf("expected 2 active rewards, got %d", len(active))
	}
	for _, r := range active {
		if !r.Active {
			t.Errorf("reward %q should be active", r.Title)
		}
	}
}

func TestRewardRedeem(t *testing.T) {
	rs, _, ms := setupRewardTestDB(t)

	member, _ := ms.Create("Alice", "#FF0000", "A")
	reward, _ := rs.Create("Treat", "", 25, true)

	redemption, err := rs.Redeem(reward.ID, &member.ID, 25)
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	if redemption.RewardID != reward.ID {
		t.Errorf("reward_id = %d, want %d", redemption.RewardID, reward.ID)
	}
	if redemption.RedeemedBy == nil || *redemption.RedeemedBy != member.ID {
		t.Errorf("redeemed_by = %v, want %d", redemption.RedeemedBy, member.ID)
	}
	if redemption.PointsSpent != 25 {
		t.Errorf("points_spent = %d, want 25", redemption.PointsSpent)
	}
}

func TestRewardListRedemptionsByMember(t *testing.T) {
	rs, _, ms := setupRewardTestDB(t)

	alice, _ := ms.Create("Alice", "#FF0000", "A")
	bob, _ := ms.Create("Bob", "#0000FF", "B")
	reward, _ := rs.Create("Treat", "", 25, true)

	rs.Redeem(reward.ID, &alice.ID, 25)
	rs.Redeem(reward.ID, &alice.ID, 25)
	rs.Redeem(reward.ID, &bob.ID, 25)

	aliceRedemptions, err := rs.ListRedemptionsByMember(alice.ID)
	if err != nil {
		t.Fatalf("list alice redemptions: %v", err)
	}
	if len(aliceRedemptions) != 2 {
		t.Fatalf("expected 2 alice redemptions, got %d", len(aliceRedemptions))
	}

	bobRedemptions, _ := rs.ListRedemptionsByMember(bob.ID)
	if len(bobRedemptions) != 1 {
		t.Fatalf("expected 1 bob redemption, got %d", len(bobRedemptions))
	}
}

func TestPointBalance(t *testing.T) {
	rs, cs, ms := setupRewardTestDB(t)

	member, _ := ms.Create("Alice", "#FF0000", "A")
	chore, _ := cs.Create("Chore A", "", nil, 10, "", nil)
	reward, _ := rs.Create("Treat", "", 15, true)

	// Earn 10+10 = 20 points
	cs.CreateCompletion(chore.ID, &member.ID, 10)
	cs.CreateCompletion(chore.ID, &member.ID, 10)

	// Spend 15 points
	rs.Redeem(reward.ID, &member.ID, 15)

	balance, err := rs.GetPointBalance(member.ID)
	if err != nil {
		t.Fatalf("get point balance: %v", err)
	}
	if balance.TotalEarned != 20 {
		t.Errorf("total_earned = %d, want 20", balance.TotalEarned)
	}
	if balance.TotalSpent != 15 {
		t.Errorf("total_spent = %d, want 15", balance.TotalSpent)
	}
	if balance.Balance != 5 {
		t.Errorf("balance = %d, want 5", balance.Balance)
	}
	if balance.MemberName != "Alice" {
		t.Errorf("member_name = %q, want %q", balance.MemberName, "Alice")
	}
}

func TestPointBalanceNoActivity(t *testing.T) {
	rs, _, ms := setupRewardTestDB(t)

	member, _ := ms.Create("NewMember", "#00FF00", "N")

	balance, err := rs.GetPointBalance(member.ID)
	if err != nil {
		t.Fatalf("get point balance: %v", err)
	}
	if balance.TotalEarned != 0 {
		t.Errorf("total_earned = %d, want 0", balance.TotalEarned)
	}
	if balance.TotalSpent != 0 {
		t.Errorf("total_spent = %d, want 0", balance.TotalSpent)
	}
	if balance.Balance != 0 {
		t.Errorf("balance = %d, want 0", balance.Balance)
	}
}

func TestGetAllPointBalances(t *testing.T) {
	rs, cs, ms := setupRewardTestDB(t)

	alice, _ := ms.Create("Alice", "#FF0000", "A")
	bob, _ := ms.Create("Bob", "#0000FF", "B")
	chore, _ := cs.Create("Chore", "", nil, 10, "", nil)

	// Alice earns 30, Bob earns 10
	cs.CreateCompletion(chore.ID, &alice.ID, 10)
	cs.CreateCompletion(chore.ID, &alice.ID, 10)
	cs.CreateCompletion(chore.ID, &alice.ID, 10)
	cs.CreateCompletion(chore.ID, &bob.ID, 10)

	balances, err := rs.GetAllPointBalances()
	if err != nil {
		t.Fatalf("get all balances: %v", err)
	}
	if len(balances) != 2 {
		t.Fatalf("expected 2 balances, got %d", len(balances))
	}

	// Should be ordered by balance DESC: Alice (30) first, Bob (10) second
	if balances[0].MemberName != "Alice" {
		t.Errorf("balances[0].MemberName = %q, want %q", balances[0].MemberName, "Alice")
	}
	if balances[0].Balance != 30 {
		t.Errorf("balances[0].Balance = %d, want 30", balances[0].Balance)
	}
	if balances[1].MemberName != "Bob" {
		t.Errorf("balances[1].MemberName = %q, want %q", balances[1].MemberName, "Bob")
	}
	if balances[1].Balance != 10 {
		t.Errorf("balances[1].Balance = %d, want 10", balances[1].Balance)
	}
}

func TestDeleteRewardCascadesRedemptions(t *testing.T) {
	rs, _, ms := setupRewardTestDB(t)

	member, _ := ms.Create("Alice", "#FF0000", "A")
	reward, _ := rs.Create("Treat", "", 25, true)
	rs.Redeem(reward.ID, &member.ID, 25)

	redemptions, _ := rs.ListRedemptionsByMember(member.ID)
	if len(redemptions) != 1 {
		t.Fatalf("expected 1 redemption, got %d", len(redemptions))
	}

	// Delete reward should cascade
	if err := rs.Delete(reward.ID); err != nil {
		t.Fatalf("delete reward: %v", err)
	}

	redemptions, _ = rs.ListRedemptionsByMember(member.ID)
	if len(redemptions) != 0 {
		t.Errorf("expected 0 redemptions after cascade, got %d", len(redemptions))
	}
}

func TestDeleteMemberSetsNullOnRedemption(t *testing.T) {
	rs, _, ms := setupRewardTestDB(t)

	member, _ := ms.Create("Alice", "#FF0000", "A")
	reward, _ := rs.Create("Treat", "", 25, true)
	rs.Redeem(reward.ID, &member.ID, 25)

	if err := ms.Delete(member.ID); err != nil {
		t.Fatalf("delete member: %v", err)
	}

	// Redemption should still exist with null redeemed_by
	// We can't query by member since they're now null, so query the reward's redemptions
	var count int
	rs.db.QueryRow(`SELECT COUNT(*) FROM reward_redemptions WHERE reward_id = ?`, reward.ID).Scan(&count)
	if count != 1 {
		t.Fatalf("expected 1 redemption, got %d", count)
	}

	var redeemedBy *int64
	var nullID sql.NullInt64
	rs.db.QueryRow(`SELECT redeemed_by FROM reward_redemptions WHERE reward_id = ?`, reward.ID).Scan(&nullID)
	if nullID.Valid {
		redeemedBy = &nullID.Int64
	}
	if redeemedBy != nil {
		t.Errorf("redeemed_by should be nil after member delete, got %v", *redeemedBy)
	}
}

func TestLeaderboardSettingSeedData(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	ss := NewSettingsStore(db)
	val, err := ss.Get("rewards_leaderboard_enabled")
	if err != nil {
		t.Fatalf("get setting: %v", err)
	}
	if val != "true" {
		t.Errorf("leaderboard setting = %q, want %q", val, "true")
	}
}
