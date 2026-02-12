package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	stripe "github.com/stripe/stripe-go/v84"

	billingstripe "github.com/dukerupert/gamwich/internal/billing/stripe"
	"github.com/dukerupert/gamwich/internal/billing/store"
)

type WebhookHandler struct {
	stripeClient      *billingstripe.Client
	accountStore      *store.AccountStore
	subscriptionStore *store.SubscriptionStore
	licenseKeyStore   *store.LicenseKeyStore
	logger            *slog.Logger
}

func NewWebhookHandler(
	sc *billingstripe.Client,
	as *store.AccountStore,
	ss *store.SubscriptionStore,
	lks *store.LicenseKeyStore,
	logger *slog.Logger,
) *WebhookHandler {
	return &WebhookHandler{
		stripeClient:      sc,
		accountStore:      as,
		subscriptionStore: ss,
		licenseKeyStore:   lks,
		logger:            logger,
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
		h.logger.Error("unmarshal checkout session", "error", err)
		return
	}

	email := sess.CustomerDetails.Email
	if email == "" {
		h.logger.Error("checkout session missing email")
		return
	}

	// Find or create account
	account, err := h.accountStore.GetByEmail(email)
	if err != nil {
		h.logger.Error("get account by email", "error", err)
		return
	}
	if account == nil {
		account, err = h.accountStore.Create(email)
		if err != nil {
			h.logger.Error("create account", "error", err)
			return
		}
	}

	// Update Stripe customer ID
	if sess.Customer != nil {
		if err := h.accountStore.UpdateStripeCustomerID(account.ID, sess.Customer.ID); err != nil {
			h.logger.Error("update stripe customer id", "error", err)
		}
	}

	// Create subscription
	sub, err := h.subscriptionStore.Create(account.ID, "cloud")
	if err != nil {
		h.logger.Error("create subscription", "error", err)
		return
	}

	if sess.Subscription != nil {
		if err := h.subscriptionStore.UpdateStripeID(sub.ID, sess.Subscription.ID); err != nil {
			h.logger.Error("update stripe subscription id", "error", err)
		}
	}

	// Generate license key
	features := "tunnel,backup,push"
	_, err = h.licenseKeyStore.Create(account.ID, sub.ID, "cloud", features)
	if err != nil {
		h.logger.Error("create license key", "error", err)
		return
	}

	h.logger.Info("checkout completed", "email", email)
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
		h.logger.Error("unmarshal invoice", "error", err)
		return
	}

	subID := getSubscriptionIDFromInvoice(invoice)
	if subID == "" {
		return
	}

	sub, err := h.subscriptionStore.GetByStripeID(subID)
	if err != nil || sub == nil {
		h.logger.Error("get subscription for invoice.paid", "error", err)
		return
	}

	if err := h.subscriptionStore.UpdateStatus(sub.ID, "active"); err != nil {
		h.logger.Error("update subscription status", "error", err)
	}

	// Extend license key expiry based on invoice period end
	lk, err := h.licenseKeyStore.GetBySubscriptionID(sub.ID)
	if err != nil || lk == nil {
		return
	}
	newExpiry := time.Unix(invoice.PeriodEnd, 0).UTC().Add(7 * 24 * time.Hour) // period end + 7 day buffer
	if err := h.licenseKeyStore.UpdateExpiry(lk.ID, newExpiry); err != nil {
		h.logger.Error("update license expiry", "error", err)
	}
}

func (h *WebhookHandler) handleInvoicePaymentFailed(event stripe.Event) {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		h.logger.Error("unmarshal invoice", "error", err)
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
		h.logger.Error("update subscription status to past_due", "error", err)
	}
}

func (h *WebhookHandler) handleSubscriptionUpdated(event stripe.Event) {
	var stripeSub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &stripeSub); err != nil {
		h.logger.Error("unmarshal subscription", "error", err)
		return
	}

	sub, err := h.subscriptionStore.GetByStripeID(stripeSub.ID)
	if err != nil || sub == nil {
		return
	}

	if err := h.subscriptionStore.UpdateStatus(sub.ID, string(stripeSub.Status)); err != nil {
		h.logger.Error("update subscription status", "error", err)
	}

	if err := h.subscriptionStore.SetCancelAtPeriodEnd(sub.ID, stripeSub.CancelAtPeriodEnd); err != nil {
		h.logger.Error("set cancel at period end", "error", err)
	}
}

func (h *WebhookHandler) handleSubscriptionDeleted(event stripe.Event) {
	var stripeSub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &stripeSub); err != nil {
		h.logger.Error("unmarshal subscription", "error", err)
		return
	}

	sub, err := h.subscriptionStore.GetByStripeID(stripeSub.ID)
	if err != nil || sub == nil {
		return
	}

	if err := h.subscriptionStore.UpdateStatus(sub.ID, "canceled"); err != nil {
		h.logger.Error("update subscription status to canceled", "error", err)
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
		h.logger.Error("update license expiry on cancel", "error", err)
	}
}
