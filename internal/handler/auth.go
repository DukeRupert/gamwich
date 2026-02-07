package handler

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/dukerupert/gamwich/internal/auth"
	"github.com/dukerupert/gamwich/internal/email"
	"github.com/dukerupert/gamwich/internal/store"
)

const sessionCookieName = "gamwich_session"

type AuthHandler struct {
	userStore      *store.UserStore
	householdStore *store.HouseholdStore
	sessionStore   *store.SessionStore
	magicLinkStore *store.MagicLinkStore
	emailClient    *email.Client
	baseURL        string
	templates      *template.Template
	logger         *slog.Logger
}

func NewAuthHandler(
	us *store.UserStore,
	hs *store.HouseholdStore,
	ss *store.SessionStore,
	mls *store.MagicLinkStore,
	ec *email.Client,
	baseURL string,
	logger *slog.Logger,
) *AuthHandler {
	tmpl := template.Must(template.ParseGlob("web/templates/auth_*.html"))
	return &AuthHandler{
		userStore:      us,
		householdStore: hs,
		sessionStore:   ss,
		magicLinkStore: mls,
		emailClient:    ec,
		baseURL:        baseURL,
		templates:      tmpl,
		logger:         logger,
	}
}

func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "auth_login.html", nil)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	emailAddr := strings.TrimSpace(r.FormValue("email"))
	if emailAddr == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	// Always show "check your email" to prevent user enumeration
	defer func() {
		h.templates.ExecuteTemplate(w, "auth_check_email.html", map[string]string{
			"Email": emailAddr,
		})
	}()

	user, err := h.userStore.GetByEmail(emailAddr)
	if err != nil {
		h.logger.Error("login lookup", "error", err)
		return
	}
	if user == nil {
		return // user doesn't exist, but we still show "check email"
	}

	// Find user's households to determine which one to use
	households, err := h.householdStore.ListHouseholdsForUser(user.ID)
	if err != nil || len(households) == 0 {
		h.logger.Error("login households", "error", err)
		return
	}

	ml, err := h.magicLinkStore.Create(emailAddr, "login", nil)
	if err != nil {
		h.logger.Error("create magic link", "error", err)
		return
	}

	if err := h.emailClient.SendMagicLink(emailAddr, ml.Token, "login", ""); err != nil {
		h.logger.Error("send magic link", "error", err)
	}
}

func (h *AuthHandler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "auth_register.html", nil)
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	emailAddr := strings.TrimSpace(r.FormValue("email"))
	householdName := strings.TrimSpace(r.FormValue("household_name"))

	if emailAddr == "" || householdName == "" {
		http.Error(w, "Email and household name are required", http.StatusBadRequest)
		return
	}

	// Check if user already exists
	existing, err := h.userStore.GetByEmail(emailAddr)
	if err != nil {
		h.logger.Error("register lookup", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if existing != nil {
		// Show check email page even if user exists (prevent enumeration)
		h.templates.ExecuteTemplate(w, "auth_check_email.html", map[string]string{
			"Email": emailAddr,
		})
		return
	}

	// Create household
	household, err := h.householdStore.Create(householdName)
	if err != nil {
		h.logger.Error("create household", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Create user
	user, err := h.userStore.Create(emailAddr, "")
	if err != nil {
		h.logger.Error("create user", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Add user as admin
	if _, err := h.householdStore.AddMember(household.ID, user.ID, "admin"); err != nil {
		h.logger.Error("add member", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Seed defaults for the new household
	if err := h.householdStore.SeedDefaults(household.ID); err != nil {
		h.logger.Error("seed defaults", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Create magic link
	ml, err := h.magicLinkStore.Create(emailAddr, "register", &household.ID)
	if err != nil {
		h.logger.Error("create magic link", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Send email
	if err := h.emailClient.SendMagicLink(emailAddr, ml.Token, "register", householdName); err != nil {
		h.logger.Error("send magic link", "error", err)
	}

	h.templates.ExecuteTemplate(w, "auth_check_email.html", map[string]string{
		"Email": emailAddr,
	})
}

func (h *AuthHandler) Verify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Invalid link", http.StatusBadRequest)
		return
	}

	ml, err := h.magicLinkStore.GetByToken(token)
	if err != nil {
		h.logger.Error("verify magic link", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if ml == nil {
		http.Error(w, "This link has expired or already been used", http.StatusBadRequest)
		return
	}

	// Mark the link as used
	if err := h.magicLinkStore.MarkUsed(ml.ID); err != nil {
		h.logger.Error("mark used", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Find the user
	user, err := h.userStore.GetByEmail(ml.Email)
	if err != nil || user == nil {
		h.logger.Error("verify user lookup", "error", err)
		http.Error(w, "User not found", http.StatusBadRequest)
		return
	}

	// Determine household
	households, err := h.householdStore.ListHouseholdsForUser(user.ID)
	if err != nil || len(households) == 0 {
		h.logger.Error("verify households", "error", err)
		http.Error(w, "No household found", http.StatusBadRequest)
		return
	}

	// Use magic link's household if specified, otherwise first household
	householdID := households[0].ID
	if ml.HouseholdID != nil {
		householdID = *ml.HouseholdID
	}

	// Create session
	sess, err := h.sessionStore.Create(user.ID, householdID)
	if err != nil {
		h.logger.Error("create session", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sess.Token,
		Path:     "/",
		MaxAge:   90 * 24 * 60 * 60, // 90 days
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})

	// Redirect
	if len(households) > 1 {
		http.Redirect(w, r, "/households", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func (h *AuthHandler) InviteAccept(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Invalid link", http.StatusBadRequest)
		return
	}

	ml, err := h.magicLinkStore.GetByToken(token)
	if err != nil {
		h.logger.Error("invite accept", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if ml == nil || ml.Purpose != "invite" || ml.HouseholdID == nil {
		http.Error(w, "This invitation link has expired or already been used", http.StatusBadRequest)
		return
	}

	// Mark as used
	if err := h.magicLinkStore.MarkUsed(ml.ID); err != nil {
		h.logger.Error("mark used", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Find or create user
	user, err := h.userStore.GetByEmail(ml.Email)
	if err != nil {
		h.logger.Error("invite user lookup", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if user == nil {
		user, err = h.userStore.Create(ml.Email, "")
		if err != nil {
			h.logger.Error("create invite user", "error", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
	}

	// Add to household (ignore error if already member)
	if _, err := h.householdStore.AddMember(*ml.HouseholdID, user.ID, "member"); err != nil {
		// Check if already a member
		existing, _ := h.householdStore.GetMember(*ml.HouseholdID, user.ID)
		if existing == nil {
			h.logger.Error("add invite member", "error", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
	}

	// Create session
	sess, err := h.sessionStore.Create(user.ID, *ml.HouseholdID)
	if err != nil {
		h.logger.Error("create invite session", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sess.Token,
		Path:     "/",
		MaxAge:   90 * 24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) Invite(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.FromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if ac.Role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	emailAddr := strings.TrimSpace(r.FormValue("email"))
	if emailAddr == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	household, err := h.householdStore.GetByID(ac.HouseholdID)
	if err != nil || household == nil {
		http.Error(w, "Household not found", http.StatusInternalServerError)
		return
	}

	ml, err := h.magicLinkStore.Create(emailAddr, "invite", &ac.HouseholdID)
	if err != nil {
		h.logger.Error("create invite link", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	if err := h.emailClient.SendMagicLink(emailAddr, ml.Token, "invite", household.Name); err != nil {
		h.logger.Error("send invite email", "error", err)
		http.Error(w, "Failed to send invitation", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Invitation sent to %s", emailAddr)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Delete session if authenticated
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		if sess, err := h.sessionStore.GetByToken(cookie.Value); err == nil && sess != nil {
			h.sessionStore.Delete(sess.ID)
		}
	}

	// Clear cookies
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     activeUserCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *AuthHandler) HouseholdsPage(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	households, err := h.householdStore.ListHouseholdsForUser(ac.UserID)
	if err != nil {
		h.logger.Error("list households", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Households":      households,
		"CurrentHousehold": ac.HouseholdID,
	}
	h.templates.ExecuteTemplate(w, "auth_households.html", data)
}

func (h *AuthHandler) SwitchHousehold(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	householdIDStr := r.FormValue("household_id")
	if householdIDStr == "" {
		http.Error(w, "Household ID required", http.StatusBadRequest)
		return
	}

	var householdID int64
	if _, err := fmt.Sscanf(householdIDStr, "%d", &householdID); err != nil {
		http.Error(w, "Invalid household ID", http.StatusBadRequest)
		return
	}

	// Verify membership
	member, err := h.householdStore.GetMember(householdID, ac.UserID)
	if err != nil || member == nil {
		http.Error(w, "Not a member of this household", http.StatusForbidden)
		return
	}

	// Update session
	if err := h.sessionStore.UpdateHouseholdID(ac.SessionID, householdID); err != nil {
		h.logger.Error("switch household", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Clear active user cookie (different household = different family members)
	http.SetCookie(w, &http.Cookie{
		Name:     activeUserCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
