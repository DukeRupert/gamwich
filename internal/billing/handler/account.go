package handler

import (
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/dukerupert/gamwich/internal/billing/model"
	"github.com/dukerupert/gamwich/internal/billing/store"
)

type AccountHandler struct {
	accountStore      *store.AccountStore
	subscriptionStore *store.SubscriptionStore
	licenseKeyStore   *store.LicenseKeyStore
	templates         map[string]*template.Template
	baseURL           string
	logger            *slog.Logger
}

func NewAccountHandler(
	as *store.AccountStore,
	ss *store.SubscriptionStore,
	lks *store.LicenseKeyStore,
	tmpl map[string]*template.Template,
	baseURL string,
	logger *slog.Logger,
) *AccountHandler {
	return &AccountHandler{
		accountStore:      as,
		subscriptionStore: ss,
		licenseKeyStore:   lks,
		templates:         tmpl,
		baseURL:           baseURL,
		logger:            logger,
	}
}

// Dashboard renders the account dashboard.
func (h *AccountHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	accountID := AccountIDFromContext(r.Context())

	account, err := h.accountStore.GetByID(accountID)
	if err != nil || account == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	sub, _ := h.subscriptionStore.GetByAccountID(accountID)

	var lk *model.LicenseKey
	if sub != nil {
		lk, _ = h.licenseKeyStore.GetBySubscriptionID(sub.ID)
	}

	data := map[string]any{
		"Account":      account,
		"Subscription": sub,
		"LicenseKey":   lk,
		"BaseURL":      h.baseURL,
		"Year":         time.Now().Year(),
		"ActiveNav":    "account",
		"AccountEmail": account.Email,
	}
	h.render(w, "account.html", data)
}

// PricingPage renders the pricing page.
func (h *AccountHandler) PricingPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"BaseURL":         h.baseURL,
		"Year":            time.Now().Year(),
		"ActiveNav":       "pricing",
		"WaitlistStatus":  r.URL.Query().Get("waitlist"),
		"IsAuthenticated": false,
		"HasSubscription": false,
	}

	accountID := AccountIDFromContext(r.Context())
	if accountID != 0 {
		data["IsAuthenticated"] = true
		if account, err := h.accountStore.GetByID(accountID); err == nil && account != nil {
			data["AccountEmail"] = account.Email
		}
		if sub, err := h.subscriptionStore.GetByAccountID(accountID); err == nil && sub != nil {
			data["HasSubscription"] = true
		}
	}

	h.render(w, "pricing.html", data)
}

func (h *AccountHandler) render(w http.ResponseWriter, name string, data any) {
	if m, ok := data.(map[string]any); ok {
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
