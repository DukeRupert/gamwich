package handler

import (
	"html/template"
	"log/slog"
	"net/http"
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
	templates    *template.Template
	logger       *slog.Logger
}

func NewAuthHandler(
	as *store.AccountStore,
	ss *store.SessionStore,
	ec *email.Client,
	baseURL string,
	tmpl *template.Template,
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
	h.render(w, "login.html", nil)
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

	http.Redirect(w, r, "/account", http.StatusSeeOther)
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if data == nil {
		data = map[string]any{"BaseURL": h.baseURL, "Year": time.Now().Year()}
	} else if m, ok := data.(map[string]any); ok {
		m["BaseURL"] = h.baseURL
		m["Year"] = time.Now().Year()
	}
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		h.logger.Error("template render", "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
