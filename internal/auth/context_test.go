package auth

import (
	"context"
	"testing"
)

func TestWithAuthAndFromContext(t *testing.T) {
	ac := AuthContext{
		UserID:      1,
		HouseholdID: 2,
		Role:        "admin",
		SessionID:   3,
	}

	ctx := WithAuth(context.Background(), ac)
	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("expected AuthContext in context")
	}
	if got.UserID != 1 {
		t.Errorf("UserID = %d, want 1", got.UserID)
	}
	if got.HouseholdID != 2 {
		t.Errorf("HouseholdID = %d, want 2", got.HouseholdID)
	}
	if got.Role != "admin" {
		t.Errorf("Role = %q, want %q", got.Role, "admin")
	}
	if got.SessionID != 3 {
		t.Errorf("SessionID = %d, want 3", got.SessionID)
	}
}

func TestFromContextMissing(t *testing.T) {
	_, ok := FromContext(context.Background())
	if ok {
		t.Error("expected false for missing AuthContext")
	}
}

func TestHouseholdID(t *testing.T) {
	ac := AuthContext{HouseholdID: 42}
	ctx := WithAuth(context.Background(), ac)
	if HouseholdID(ctx) != 42 {
		t.Errorf("HouseholdID = %d, want 42", HouseholdID(ctx))
	}
}

func TestHouseholdIDMissing(t *testing.T) {
	if HouseholdID(context.Background()) != 0 {
		t.Error("expected 0 for missing context")
	}
}

func TestUserID(t *testing.T) {
	ac := AuthContext{UserID: 7}
	ctx := WithAuth(context.Background(), ac)
	if UserID(ctx) != 7 {
		t.Errorf("UserID = %d, want 7", UserID(ctx))
	}
}

func TestUserIDMissing(t *testing.T) {
	if UserID(context.Background()) != 0 {
		t.Error("expected 0 for missing context")
	}
}

func TestIsAdmin(t *testing.T) {
	ctx := WithAuth(context.Background(), AuthContext{Role: "admin"})
	if !IsAdmin(ctx) {
		t.Error("expected IsAdmin = true for admin role")
	}
}

func TestIsAdminFalse(t *testing.T) {
	ctx := WithAuth(context.Background(), AuthContext{Role: "member"})
	if IsAdmin(ctx) {
		t.Error("expected IsAdmin = false for member role")
	}
}

func TestIsAdminMissing(t *testing.T) {
	if IsAdmin(context.Background()) {
		t.Error("expected IsAdmin = false for missing context")
	}
}
