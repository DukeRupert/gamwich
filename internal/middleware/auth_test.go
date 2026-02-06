package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dukerupert/gamwich/internal/auth"
	"github.com/dukerupert/gamwich/internal/database"
	"github.com/dukerupert/gamwich/internal/store"
)

func setupAuthMiddlewareDB(t *testing.T) (*store.SessionStore, *store.HouseholdStore, *store.UserStore) {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return store.NewSessionStore(db), store.NewHouseholdStore(db), store.NewUserStore(db)
}

func TestRequireAuthNoCookie(t *testing.T) {
	ss, hs, _ := setupAuthMiddlewareDB(t)

	handler := RequireAuth(ss, hs)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want %q", loc, "/login")
	}
}

func TestRequireAuthInvalidToken(t *testing.T) {
	ss, hs, _ := setupAuthMiddlewareDB(t)

	handler := RequireAuth(ss, hs)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "invalid-token"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
}

func TestRequireAuthValidSession(t *testing.T) {
	ss, hs, us := setupAuthMiddlewareDB(t)

	u, _ := us.Create("alice@example.com", "Alice")
	hs.AddMember(1, u.ID, "admin") // default household from migration
	sess, _ := ss.Create(u.ID, 1)

	var gotAC auth.AuthContext
	handler := RequireAuth(ss, hs)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ac, ok := auth.FromContext(r.Context())
		if !ok {
			t.Fatal("expected AuthContext in request context")
		}
		gotAC = ac
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sess.Token})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotAC.UserID != u.ID {
		t.Errorf("UserID = %d, want %d", gotAC.UserID, u.ID)
	}
	if gotAC.HouseholdID != 1 {
		t.Errorf("HouseholdID = %d, want 1", gotAC.HouseholdID)
	}
	if gotAC.Role != "admin" {
		t.Errorf("Role = %q, want %q", gotAC.Role, "admin")
	}
}

func TestRequireAuthHTMXRedirect(t *testing.T) {
	ss, hs, _ := setupAuthMiddlewareDB(t)

	handler := RequireAuth(ss, hs)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if hxRedirect := rec.Header().Get("HX-Redirect"); hxRedirect != "/login" {
		t.Errorf("HX-Redirect = %q, want %q", hxRedirect, "/login")
	}
}

func TestRequireAdminAllowed(t *testing.T) {
	ctx := auth.WithAuth(context.Background(), auth.AuthContext{Role: "admin"})
	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler := RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRequireAdminForbidden(t *testing.T) {
	ctx := auth.WithAuth(context.Background(), auth.AuthContext{Role: "member"})
	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler := RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
