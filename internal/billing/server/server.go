package server

import (
	"database/sql"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/dukerupert/gamwich/internal/billing/handler"
	"github.com/dukerupert/gamwich/internal/billing/middleware"
	"github.com/dukerupert/gamwich/internal/billing/store"
	billingstripe "github.com/dukerupert/gamwich/internal/billing/stripe"
	"github.com/dukerupert/gamwich/internal/email"
	sharedmw "github.com/dukerupert/gamwich/internal/middleware"
)

type Server struct {
	db                *sql.DB
	accountStore      *store.AccountStore
	subscriptionStore *store.SubscriptionStore
	licenseKeyStore   *store.LicenseKeyStore
	sessionStore      *store.SessionStore
	webhookH          *handler.WebhookHandler
	checkoutH         *handler.CheckoutHandler
	authH             *handler.AuthHandler
	accountH          *handler.AccountHandler
	stripeClient      *billingstripe.Client
	rateLimiter       *sharedmw.RateLimiter
}

type Config struct {
	Stripe       billingstripe.Config
	BaseURL      string
	EmailClient  *email.Client
	TemplatesDir string
}

func New(db *sql.DB, cfg Config, logger *slog.Logger) *Server {
	accountStore := store.NewAccountStore(db)
	subscriptionStore := store.NewSubscriptionStore(db)
	licenseKeyStore := store.NewLicenseKeyStore(db)
	sessionStore := store.NewSessionStore(db)

	var stripeClient *billingstripe.Client
	if cfg.Stripe.SecretKey != "" {
		stripeClient = billingstripe.NewClient(cfg.Stripe)
	}

	var webhookH *handler.WebhookHandler
	var checkoutH *handler.CheckoutHandler
	if stripeClient != nil {
		webhookH = handler.NewWebhookHandler(stripeClient, accountStore, subscriptionStore, licenseKeyStore, logger.With("component", "webhook"))
		checkoutH = handler.NewCheckoutHandler(stripeClient, accountStore)
	}

	// Load billing templates
	tmplDir := cfg.TemplatesDir
	if tmplDir == "" {
		tmplDir = "web/billing/templates"
	}
	tmpl := template.Must(template.ParseGlob(tmplDir + "/*.html"))

	authH := handler.NewAuthHandler(accountStore, sessionStore, cfg.EmailClient, cfg.BaseURL, tmpl, logger.With("component", "auth"))
	accountH := handler.NewAccountHandler(accountStore, subscriptionStore, licenseKeyStore, tmpl, cfg.BaseURL, logger.With("component", "account"))

	return &Server{
		db:                db,
		accountStore:      accountStore,
		subscriptionStore: subscriptionStore,
		licenseKeyStore:   licenseKeyStore,
		sessionStore:      sessionStore,
		webhookH:          webhookH,
		checkoutH:         checkoutH,
		authH:             authH,
		accountH:          accountH,
		stripeClient:      stripeClient,
		rateLimiter:       sharedmw.NewRateLimiter(),
	}
}

// SessionStore returns the session store for cleanup tasks.
func (s *Server) SessionStore() *store.SessionStore {
	return s.sessionStore
}

// RateLimiter returns the rate limiter for cleanup tasks.
func (s *Server) RateLimiter() *sharedmw.RateLimiter {
	return s.rateLimiter
}

func (s *Server) Router() http.Handler {
	outerMux := http.NewServeMux()

	// Public routes
	outerMux.HandleFunc("GET /health", s.healthCheck)
	outerMux.HandleFunc("GET /login", s.authH.LoginPage)
	outerMux.HandleFunc("POST /login", s.rateLimitedHandler(s.authH.Login))
	outerMux.HandleFunc("GET /auth/verify", s.authH.Verify)
	outerMux.HandleFunc("GET /pricing", s.accountH.PricingPage)

	// Stripe webhook (public, no auth)
	if s.webhookH != nil {
		outerMux.HandleFunc("POST /webhooks/stripe", s.webhookH.HandleStripeWebhook)
	}

	// License validation (public, rate-limited)
	licenseH := handler.NewLicenseHandler(s.licenseKeyStore)
	rateLimitMw := sharedmw.RateLimit(s.rateLimiter, func(r *http.Request) string {
		return r.RemoteAddr
	}, 10, time.Minute)
	outerMux.Handle("POST /api/license/validate", rateLimitMw(http.HandlerFunc(licenseH.Validate)))

	// Static files
	outerMux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/billing/static"))))

	// Protected routes
	protectedMux := http.NewServeMux()
	protectedMux.HandleFunc("POST /logout", s.authH.Logout)
	protectedMux.HandleFunc("GET /account", s.accountH.Dashboard)

	if s.checkoutH != nil {
		protectedMux.HandleFunc("POST /api/checkout", s.checkoutH.CreateCheckoutSession)
		protectedMux.HandleFunc("POST /api/billing-portal", s.checkoutH.BillingPortal)
	}

	authMw := middleware.RequireAuth(s.sessionStore)
	outerMux.Handle("/", authMw(protectedMux))

	return outerMux
}

func (s *Server) rateLimitedHandler(h http.HandlerFunc) http.HandlerFunc {
	keyFunc := func(r *http.Request) string {
		return r.RemoteAddr
	}
	rl := sharedmw.RateLimit(s.rateLimiter, keyFunc, 10, time.Minute)
	return func(w http.ResponseWriter, r *http.Request) {
		rl(http.HandlerFunc(h)).ServeHTTP(w, r)
	}
}

func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
