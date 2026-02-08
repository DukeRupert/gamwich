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
	waitlistStore     *store.WaitlistStore
	webhookH          *handler.WebhookHandler
	checkoutH         *handler.CheckoutHandler
	authH             *handler.AuthHandler
	accountH          *handler.AccountHandler
	marketingH        *handler.MarketingHandler
	waitlistH         *handler.WaitlistHandler
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

	// Load billing templates — per-page sets to avoid {{define "content"}} collisions
	tmplDir := cfg.TemplatesDir
	if tmplDir == "" {
		tmplDir = "web/billing/templates"
	}
	layoutFile := tmplDir + "/layout.html"
	templates := make(map[string]*template.Template)
	pages := []string{"login.html", "check_email.html", "pricing.html", "account.html", "index.html", "features.html"}
	for _, page := range pages {
		templates[page] = template.Must(template.ParseFiles(layoutFile, tmplDir+"/"+page))
	}

	authH := handler.NewAuthHandler(accountStore, sessionStore, cfg.EmailClient, cfg.BaseURL, templates, logger.With("component", "auth"))
	accountH := handler.NewAccountHandler(accountStore, subscriptionStore, licenseKeyStore, templates, cfg.BaseURL, logger.With("component", "account"))
	marketingH := handler.NewMarketingHandler(templates, cfg.BaseURL, logger.With("component", "marketing"))
	waitlistStore := store.NewWaitlistStore(db)
	waitlistH := handler.NewWaitlistHandler(waitlistStore, logger.With("component", "waitlist"))

	return &Server{
		db:                db,
		accountStore:      accountStore,
		subscriptionStore: subscriptionStore,
		licenseKeyStore:   licenseKeyStore,
		sessionStore:      sessionStore,
		waitlistStore:     waitlistStore,
		webhookH:          webhookH,
		checkoutH:         checkoutH,
		authH:             authH,
		accountH:          accountH,
		marketingH:        marketingH,
		waitlistH:         waitlistH,
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
	mux := http.NewServeMux()

	// Public marketing routes
	mux.HandleFunc("GET /{$}", s.marketingH.LandingPage)
	mux.HandleFunc("GET /features", s.marketingH.FeaturesPage)
	mux.HandleFunc("GET /pricing", s.accountH.PricingPage)
	mux.HandleFunc("POST /waitlist", s.rateLimitedHandler(s.waitlistH.Join))

	// Auth routes (public)
	mux.HandleFunc("GET /health", s.healthCheck)
	mux.HandleFunc("GET /login", s.authH.LoginPage)
	mux.HandleFunc("POST /login", s.rateLimitedHandler(s.authH.Login))
	mux.HandleFunc("GET /auth/verify", s.authH.Verify)

	// Stripe webhook (public, no auth)
	if s.webhookH != nil {
		mux.HandleFunc("POST /webhooks/stripe", s.webhookH.HandleStripeWebhook)
	}

	// License validation (public, rate-limited)
	licenseH := handler.NewLicenseHandler(s.licenseKeyStore)
	rateLimitMw := sharedmw.RateLimit(s.rateLimiter, func(r *http.Request) string {
		return r.RemoteAddr
	}, 10, time.Minute)
	mux.Handle("POST /api/license/validate", rateLimitMw(http.HandlerFunc(licenseH.Validate)))

	// Static files
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/billing/static"))))

	// SEO static files
	mux.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/billing/static/robots.txt")
	})

	// Protected routes (explicitly registered with auth middleware)
	authMw := middleware.RequireAuth(s.sessionStore)
	mux.Handle("POST /logout", authMw(http.HandlerFunc(s.authH.Logout)))
	mux.Handle("GET /account", authMw(http.HandlerFunc(s.accountH.Dashboard)))

	if s.checkoutH != nil {
		mux.Handle("POST /api/checkout", authMw(http.HandlerFunc(s.checkoutH.CreateCheckoutSession)))
		mux.Handle("POST /api/billing-portal", authMw(http.HandlerFunc(s.checkoutH.BillingPortal)))
	}

	return mux
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
