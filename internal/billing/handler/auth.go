package handler

import (
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/dukerupert/gamwich/internal/billing/store"
	"github.com/dukerupert/gamwich/internal/email"
)

const billingSessionCookie = "billing_session"

type AuthHandler struct {
	accountStore *store.AccountStore
	sessionStore *store.SessionStore
	emailClient  *email.Client
	baseURL      string
	templates    map[string]*template.Template
	logger       *slog.Logger
}

func NewAuthHandler(
	as *store.AccountStore,
	ss *store.SessionStore,
	ec *email.Client,
	baseURL string,
	tmpl map[string]*template.Template,
	logger *slog.Logger,
) *AuthHandler {
	return &AuthHandler{
		accountStore: as,
		sessionStore: ss,
		emailClient:  ec,
		baseURL:      baseURL,
		templates:    tmpl,
		logger:       logger,
	}
}

// LoginPage renders the login form.
func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{}
	if redirect := r.URL.Query().Get("redirect"); redirect != "" {
		data["Redirect"] = redirect
	}
	h.render(w, "login.html", data)
}

// Login handles the magic link request.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.render(w, "login.html", map[string]any{"Error": "Invalid form data"})
		return
	}

	addr := r.FormValue("email")
	if addr == "" {
		h.render(w, "login.html", map[string]any{"Error": "Email is required"})
		return
	}

	// Find or create account
	account, err := h.accountStore.GetByEmail(addr)
	if err != nil {
		h.logger.Error("get account", "error", err)
	}
	if account == nil {
		account, err = h.accountStore.Create(addr)
		if err != nil {
			h.logger.Error("create account", "error", err)
			h.render(w, "login.html", map[string]any{"Error": "Unable to process request"})
			return
		}
	}

	// Create a session directly and send magic link with session token
	sess, err := h.sessionStore.Create(account.ID)
	if err != nil {
		h.logger.Error("create session", "error", err)
		h.render(w, "login.html", map[string]any{"Error": "Unable to process request"})
		return
	}

	// Send magic link
	if h.emailClient != nil && h.emailClient.Configured() {
		if err := h.emailClient.SendAuthCode(addr, sess.Token, "login", "Gamwich"); err != nil {
			h.logger.Error("send auth code", "error", err)
		}
	} else {
		h.logger.Info("magic link token generated", "email", addr, "token", sess.Token)
	}

	// Set redirect cookie if provided
	if redirect := r.FormValue("redirect"); redirect != "" && isValidRedirect(redirect) {
		http.SetCookie(w, &http.Cookie{
			Name:     "billing_redirect",
			Value:    redirect,
			Path:     "/",
			MaxAge:   3600, // 1 hour
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}

	// Always show check-email to prevent user enumeration
	h.render(w, "check_email.html", map[string]any{"Email": addr})
}

// Verify processes the magic link token and creates a session.
func (h *AuthHandler) Verify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		h.render(w, "login.html", map[string]any{"Error": "Invalid or expired link"})
		return
	}

	sess, err := h.sessionStore.GetByToken(token)
	if err != nil || sess == nil {
		h.render(w, "login.html", map[string]any{"Error": "Invalid or expired link"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     billingSessionCookie,
		Value:    sess.Token,
		Path:     "/",
		MaxAge:   90 * 24 * 60 * 60, // 90 days
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	// Check for redirect cookie
	redirectTarget := "/account"
	if cookie, err := r.Cookie("billing_redirect"); err == nil && cookie.Value != "" && isValidRedirect(cookie.Value) {
		redirectTarget = cookie.Value
		// Clear the redirect cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "billing_redirect",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
		})
	}

	http.Redirect(w, r, redirectTarget, http.StatusSeeOther)
}

// Logout destroys the session.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(billingSessionCookie)
	if err == nil && cookie.Value != "" {
		sess, err := h.sessionStore.GetByToken(cookie.Value)
		if err == nil && sess != nil {
			h.sessionStore.Delete(sess.ID)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     billingSessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *AuthHandler) render(w http.ResponseWriter, name string, data any) {
	if data == nil {
		data = map[string]any{"BaseURL": h.baseURL, "Year": time.Now().Year(), "ActiveNav": ""}
	} else if m, ok := data.(map[string]any); ok {
		m["BaseURL"] = h.baseURL
		m["Year"] = time.Now().Year()
		if _, exists := m["ActiveNav"]; !exists {
			m["ActiveNav"] = ""
		}
	}
	tmpl, ok := h.templates[name]
	if !ok {
		h.logger.Error("template not found", "name", name)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		h.logger.Error("template render", "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// isValidRedirect checks that a redirect path is a safe relative path.
func isValidRedirect(path string) bool {
	return strings.HasPrefix(path, "/") && !strings.Contains(path, "://")
}
