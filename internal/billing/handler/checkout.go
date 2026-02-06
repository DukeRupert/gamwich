package handler

import (
	"encoding/json"
	"net/http"

	billingstripe "github.com/dukerupert/gamwich/internal/billing/stripe"
	"github.com/dukerupert/gamwich/internal/billing/store"
)

type CheckoutHandler struct {
	stripeClient *billingstripe.Client
	accountStore *store.AccountStore
}

func NewCheckoutHandler(sc *billingstripe.Client, as *store.AccountStore) *CheckoutHandler {
	return &CheckoutHandler{
		stripeClient: sc,
		accountStore: as,
	}
}

// CreateCheckoutSession creates a Stripe checkout session and returns the URL.
func (h *CheckoutHandler) CreateCheckoutSession(w http.ResponseWriter, r *http.Request) {
	accountID := AccountIDFromContext(r.Context())
	if accountID == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Plan     string `json:"plan"`
		Interval string `json:"interval"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.Plan == "" {
		req.Plan = "cloud"
	}
	if req.Interval == "" {
		req.Interval = "monthly"
	}

	account, err := h.accountStore.GetByID(accountID)
	if err != nil || account == nil {
		http.Error(w, "account not found", http.StatusNotFound)
		return
	}

	// Ensure Stripe customer exists
	customerID := ""
	if account.StripeCustomerID != nil {
		customerID = *account.StripeCustomerID
	}
	if customerID == "" {
		customerID, err = h.stripeClient.CreateCustomer(account.Email)
		if err != nil {
			http.Error(w, "failed to create customer", http.StatusInternalServerError)
			return
		}
		h.accountStore.UpdateStripeCustomerID(account.ID, customerID)
	}

	priceID := h.stripeClient.PriceIDForPlan(req.Plan, req.Interval)
	url, err := h.stripeClient.CreateCheckoutSession(customerID, priceID)
	if err != nil {
		http.Error(w, "failed to create checkout session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": url})
}

// BillingPortal creates a Stripe billing portal session and returns the URL.
func (h *CheckoutHandler) BillingPortal(w http.ResponseWriter, r *http.Request) {
	accountID := AccountIDFromContext(r.Context())
	if accountID == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	account, err := h.accountStore.GetByID(accountID)
	if err != nil || account == nil {
		http.Error(w, "account not found", http.StatusNotFound)
		return
	}

	if account.StripeCustomerID == nil {
		http.Error(w, "no billing account", http.StatusBadRequest)
		return
	}

	returnURL := r.Header.Get("Referer")
	if returnURL == "" {
		returnURL = "/account"
	}

	url, err := h.stripeClient.CreateBillingPortalSession(*account.StripeCustomerID, returnURL)
	if err != nil {
		http.Error(w, "failed to create portal session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": url})
}
