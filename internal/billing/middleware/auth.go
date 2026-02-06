package middleware

import (
	"net/http"

	"github.com/dukerupert/gamwich/internal/billing/handler"
	"github.com/dukerupert/gamwich/internal/billing/store"
)

const sessionCookieName = "billing_session"

// RequireAuth validates the session cookie and populates account ID in context.
func RequireAuth(sessionStore *store.SessionStore) func(http.Handler) http.Handler {
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

			ctx := handler.WithAccountID(r.Context(), sess.AccountID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
