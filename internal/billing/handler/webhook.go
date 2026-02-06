package handler

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	stripe "github.com/stripe/stripe-go/v82"

	billingstripe "github.com/dukerupert/gamwich/internal/billing/stripe"
	"github.com/dukerupert/gamwich/internal/billing/store"
)

type WebhookHandler struct {
	stripeClient      *billingstripe.Client
	accountStore      *store.AccountStore
	subscriptionStore *store.SubscriptionStore
	licenseKeyStore   *store.LicenseKeyStore
}

func NewWebhookHandler(
	sc *billingstripe.Client,
	as *store.AccountStore,
	ss *store.SubscriptionStore,
	lks *store.LicenseKeyStore,
) *WebhookHandler {
	return &WebhookHandler{
		stripeClient:      sc,
		accountStore:      as,
		subscriptionStore: ss,
		licenseKeyStore:   lks,
	}
}

func (h *WebhookHandler) HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	event, err := h.stripeClient.ConstructWebhookEvent(body, r.Header.Get("Stripe-Signature"))
	if err != nil {
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	switch event.Type {
	case "checkout.session.completed":
		h.handleCheckoutCompleted(event)
	case "invoice.paid":
		h.handleInvoicePaid(event)
	case "invoice.payment_failed":
		h.handleInvoicePaymentFailed(event)
	case "customer.subscription.updated":
		h.handleSubscriptionUpdated(event)
	case "customer.subscription.deleted":
		h.handleSubscriptionDeleted(event)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) handleCheckoutCompleted(event stripe.Event) {
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		log.Printf("webhook: unmarshal checkout session: %v", err)
		return
	}

	email := sess.CustomerDetails.Email
	if email == "" {
		log.Printf("webhook: checkout session missing email")
		return
	}

	// Find or create account
	account, err := h.accountStore.GetByEmail(email)
	if err != nil {
		log.Printf("webhook: get account by email: %v", err)
		return
	}
	if account == nil {
		account, err = h.accountStore.Create(email)
		if err != nil {
			log.Printf("webhook: create account: %v", err)
			return
		}
	}

	// Update Stripe customer ID
	if sess.Customer != nil {
		if err := h.accountStore.UpdateStripeCustomerID(account.ID, sess.Customer.ID); err != nil {
			log.Printf("webhook: update stripe customer id: %v", err)
		}
	}

	// Create subscription
	sub, err := h.subscriptionStore.Create(account.ID, "cloud")
	if err != nil {
		log.Printf("webhook: create subscription: %v", err)
		return
	}

	if sess.Subscription != nil {
		if err := h.subscriptionStore.UpdateStripeID(sub.ID, sess.Subscription.ID); err != nil {
			log.Printf("webhook: update stripe subscription id: %v", err)
		}
	}

	// Generate license key
	features := "tunnel,backup,push"
	_, err = h.licenseKeyStore.Create(account.ID, sub.ID, "cloud", features)
	if err != nil {
		log.Printf("webhook: create license key: %v", err)
		return
	}

	log.Printf("webhook: checkout completed for %s", email)
}

// getSubscriptionIDFromInvoice extracts the subscription ID from an invoice's parent.
func getSubscriptionIDFromInvoice(invoice stripe.Invoice) string {
	if invoice.Parent != nil &&
		invoice.Parent.SubscriptionDetails != nil &&
		invoice.Parent.SubscriptionDetails.Subscription != nil {
		return invoice.Parent.SubscriptionDetails.Subscription.ID
	}
	return ""
}

func (h *WebhookHandler) handleInvoicePaid(event stripe.Event) {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		log.Printf("webhook: unmarshal invoice: %v", err)
		return
	}

	subID := getSubscriptionIDFromInvoice(invoice)
	if subID == "" {
		return
	}

	sub, err := h.subscriptionStore.GetByStripeID(subID)
	if err != nil || sub == nil {
		log.Printf("webhook: get subscription for invoice.paid: %v", err)
		return
	}

	if err := h.subscriptionStore.UpdateStatus(sub.ID, "active"); err != nil {
		log.Printf("webhook: update subscription status: %v", err)
	}

	// Extend license key expiry based on invoice period end
	lk, err := h.licenseKeyStore.GetBySubscriptionID(sub.ID)
	if err != nil || lk == nil {
		return
	}
	newExpiry := time.Unix(invoice.PeriodEnd, 0).UTC().Add(7 * 24 * time.Hour) // period end + 7 day buffer
	if err := h.licenseKeyStore.UpdateExpiry(lk.ID, newExpiry); err != nil {
		log.Printf("webhook: update license expiry: %v", err)
	}
}

func (h *WebhookHandler) handleInvoicePaymentFailed(event stripe.Event) {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		log.Printf("webhook: unmarshal invoice: %v", err)
		return
	}

	subID := getSubscriptionIDFromInvoice(invoice)
	if subID == "" {
		return
	}

	sub, err := h.subscriptionStore.GetByStripeID(subID)
	if err != nil || sub == nil {
		return
	}

	if err := h.subscriptionStore.UpdateStatus(sub.ID, "past_due"); err != nil {
		log.Printf("webhook: update subscription status to past_due: %v", err)
	}
}

func (h *WebhookHandler) handleSubscriptionUpdated(event stripe.Event) {
	var stripeSub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &stripeSub); err != nil {
		log.Printf("webhook: unmarshal subscription: %v", err)
		return
	}

	sub, err := h.subscriptionStore.GetByStripeID(stripeSub.ID)
	if err != nil || sub == nil {
		return
	}

	if err := h.subscriptionStore.UpdateStatus(sub.ID, string(stripeSub.Status)); err != nil {
		log.Printf("webhook: update subscription status: %v", err)
	}

	if err := h.subscriptionStore.SetCancelAtPeriodEnd(sub.ID, stripeSub.CancelAtPeriodEnd); err != nil {
		log.Printf("webhook: set cancel at period end: %v", err)
	}
}

func (h *WebhookHandler) handleSubscriptionDeleted(event stripe.Event) {
	var stripeSub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &stripeSub); err != nil {
		log.Printf("webhook: unmarshal subscription: %v", err)
		return
	}

	sub, err := h.subscriptionStore.GetByStripeID(stripeSub.ID)
	if err != nil || sub == nil {
		return
	}

	if err := h.subscriptionStore.UpdateStatus(sub.ID, "canceled"); err != nil {
		log.Printf("webhook: update subscription status to canceled: %v", err)
	}

	// Set license expiry to cancel_at or now + 7 days
	lk, err := h.licenseKeyStore.GetBySubscriptionID(sub.ID)
	if err != nil || lk == nil {
		return
	}
	expiry := time.Now().UTC().Add(7 * 24 * time.Hour)
	if stripeSub.CancelAt > 0 {
		expiry = time.Unix(stripeSub.CancelAt, 0).UTC()
	}
	if err := h.licenseKeyStore.UpdateExpiry(lk.ID, expiry); err != nil {
		log.Printf("webhook: update license expiry on cancel: %v", err)
	}
}
