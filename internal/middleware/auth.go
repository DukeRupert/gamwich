package middleware

import (
	"net/http"

	"github.com/dukerupert/gamwich/internal/auth"
	"github.com/dukerupert/gamwich/internal/store"
)

const sessionCookieName = "gamwich_session"

// RequireAuth validates the session cookie and populates AuthContext.
// HTMX-aware: returns HX-Redirect header instead of 303 redirect for HTMX requests.
func RequireAuth(sessionStore *store.SessionStore, householdStore *store.HouseholdStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || cookie.Value == "" {
				redirectToLogin(w, r)
				return
			}

			sess, err := sessionStore.GetByToken(cookie.Value)
			if err != nil || sess == nil {
				redirectToLogin(w, r)
				return
			}

			member, err := householdStore.GetMember(sess.HouseholdID, sess.UserID)
			if err != nil || member == nil {
				redirectToLogin(w, r)
				return
			}

			ac := auth.AuthContext{
				UserID:      sess.UserID,
				HouseholdID: sess.HouseholdID,
				Role:        member.Role,
				SessionID:   sess.ID,
			}

			ctx := auth.WithAuth(r.Context(), ac)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin checks that the authenticated user has the admin role.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !auth.IsAdmin(r.Context()) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
