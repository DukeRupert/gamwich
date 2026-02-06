package handler

import (
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/dukerupert/gamwich/internal/billing/model"
	"github.com/dukerupert/gamwich/internal/billing/store"
)

type AccountHandler struct {
	accountStore      *store.AccountStore
	subscriptionStore *store.SubscriptionStore
	licenseKeyStore   *store.LicenseKeyStore
	templates         *template.Template
	baseURL           string
}

func NewAccountHandler(
	as *store.AccountStore,
	ss *store.SubscriptionStore,
	lks *store.LicenseKeyStore,
	tmpl *template.Template,
	baseURL string,
) *AccountHandler {
	return &AccountHandler{
		accountStore:      as,
		subscriptionStore: ss,
		licenseKeyStore:   lks,
		templates:         tmpl,
		baseURL:           baseURL,
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
	}
	h.render(w, "account.html", data)
}

// PricingPage renders the pricing page.
func (h *AccountHandler) PricingPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"BaseURL": h.baseURL,
		"Year":    time.Now().Year(),
	}
	h.render(w, "pricing.html", data)
}

func (h *AccountHandler) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("billing template error: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
