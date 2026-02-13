package stripe

import (
	"fmt"

	stripe "github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/billingportal/session"
	checksession "github.com/stripe/stripe-go/v84/checkout/session"
	"github.com/stripe/stripe-go/v84/customer"
	"github.com/stripe/stripe-go/v84/webhook"
)

type Config struct {
	SecretKey         string
	WebhookSecret     string
	CloudPriceID      string
	CloudAnnualPriceID string
	SuccessURL        string
	CancelURL         string
}

type Client struct {
	cfg Config
}

func NewClient(cfg Config) *Client {
	stripe.Key = cfg.SecretKey
	return &Client{cfg: cfg}
}

// CreateCustomer creates a Stripe customer and returns the customer ID.
func (c *Client) CreateCustomer(email string) (string, error) {
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
	}
	cust, err := customer.New(params)
	if err != nil {
		return "", fmt.Errorf("create stripe customer: %w", err)
	}
	return cust.ID, nil
}

// CreateCheckoutSession creates a Stripe checkout session and returns the URL.
func (c *Client) CreateCheckoutSession(customerID, priceID string) (string, error) {
	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		AllowPromotionCodes: stripe.Bool(true),
		SuccessURL:          stripe.String(c.cfg.SuccessURL),
		CancelURL:           stripe.String(c.cfg.CancelURL),
	}
	sess, err := checksession.New(params)
	if err != nil {
		return "", fmt.Errorf("create checkout session: %w", err)
	}
	return sess.URL, nil
}

// CreateBillingPortalSession creates a Stripe billing portal session and returns the URL.
func (c *Client) CreateBillingPortalSession(customerID, returnURL string) (string, error) {
	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(customerID),
		ReturnURL: stripe.String(returnURL),
	}
	sess, err := session.New(params)
	if err != nil {
		return "", fmt.Errorf("create billing portal session: %w", err)
	}
	return sess.URL, nil
}

// PriceIDForPlan returns the Stripe price ID for the given plan and interval.
func (c *Client) PriceIDForPlan(plan, interval string) string {
	if plan == "cloud" && interval == "annual" {
		return c.cfg.CloudAnnualPriceID
	}
	return c.cfg.CloudPriceID
}

// ConstructWebhookEvent verifies the signature and returns the parsed event.
func (c *Client) ConstructWebhookEvent(payload []byte, sigHeader string) (stripe.Event, error) {
	return webhook.ConstructEvent(payload, sigHeader, c.cfg.WebhookSecret)
}
