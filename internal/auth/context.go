package auth

import "context"

type contextKey struct{}

type AuthContext struct {
	UserID      int64
	HouseholdID int64
	Role        string
	SessionID   int64
}

func WithAuth(ctx context.Context, ac AuthContext) context.Context {
	return context.WithValue(ctx, contextKey{}, ac)
}

func FromContext(ctx context.Context) (AuthContext, bool) {
	ac, ok := ctx.Value(contextKey{}).(AuthContext)
	return ac, ok
}

func HouseholdID(ctx context.Context) int64 {
	ac, ok := FromContext(ctx)
	if !ok {
		return 0
	}
	return ac.HouseholdID
}

func UserID(ctx context.Context) int64 {
	ac, ok := FromContext(ctx)
	if !ok {
		return 0
	}
	return ac.UserID
}

func IsAdmin(ctx context.Context) bool {
	ac, ok := FromContext(ctx)
	if !ok {
		return false
	}
	return ac.Role == "admin"
}
