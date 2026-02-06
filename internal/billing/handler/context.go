package handler

import "context"

type contextKey struct{}

// WithAccountID stores the account ID in the context.
func WithAccountID(ctx context.Context, accountID int64) context.Context {
	return context.WithValue(ctx, contextKey{}, accountID)
}

// AccountIDFromContext retrieves the account ID from the context.
func AccountIDFromContext(ctx context.Context) int64 {
	id, _ := ctx.Value(contextKey{}).(int64)
	return id
}
