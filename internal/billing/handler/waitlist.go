package handler

import (
	"log/slog"
	"net/http"
	"net/mail"

	"github.com/dukerupert/gamwich/internal/billing/store"
)

type WaitlistHandler struct {
	waitlistStore *store.WaitlistStore
	logger        *slog.Logger
}

func NewWaitlistHandler(ws *store.WaitlistStore, logger *slog.Logger) *WaitlistHandler {
	return &WaitlistHandler{
		waitlistStore: ws,
		logger:        logger,
	}
}

// Join handles waitlist signup from the pricing page.
func (h *WaitlistHandler) Join(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/pricing?waitlist=error", http.StatusSeeOther)
		return
	}

	email := r.FormValue("email")
	if email == "" {
		http.Redirect(w, r, "/pricing?waitlist=error", http.StatusSeeOther)
		return
	}

	// Basic email validation
	if _, err := mail.ParseAddress(email); err != nil {
		http.Redirect(w, r, "/pricing?waitlist=error", http.StatusSeeOther)
		return
	}

	plan := r.FormValue("plan")
	if plan == "" {
		plan = "hosted"
	}

	if err := h.waitlistStore.Create(email, plan); err != nil {
		h.logger.Error("waitlist create", "error", err)
		http.Redirect(w, r, "/pricing?waitlist=error", http.StatusSeeOther)
		return
	}

	h.logger.Info("waitlist signup", "email", email, "plan", plan)
	http.Redirect(w, r, "/pricing?waitlist=success", http.StatusSeeOther)
}
