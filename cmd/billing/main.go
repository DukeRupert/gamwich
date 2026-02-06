package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dukerupert/gamwich/internal/billing/database"
	"github.com/dukerupert/gamwich/internal/billing/server"
	billingstripe "github.com/dukerupert/gamwich/internal/billing/stripe"
	"github.com/dukerupert/gamwich/internal/email"
)

func main() {
	port := os.Getenv("BILLING_PORT")
	if port == "" {
		port = "8090"
	}

	dbPath := os.Getenv("BILLING_DB_PATH")
	if dbPath == "" {
		dbPath = "billing.db"
	}

	baseURL := os.Getenv("BILLING_BASE_URL")
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%s", port)
	}

	db, err := database.Open(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Email config
	postmarkToken := os.Getenv("BILLING_POSTMARK_TOKEN")
	fromEmail := os.Getenv("BILLING_FROM_EMAIL")
	emailClient := email.NewClient(postmarkToken, fromEmail, baseURL)

	cfg := server.Config{
		Stripe: billingstripe.Config{
			SecretKey:          os.Getenv("STRIPE_SECRET_KEY"),
			WebhookSecret:     os.Getenv("STRIPE_WEBHOOK_SECRET"),
			CloudPriceID:      os.Getenv("STRIPE_CLOUD_PRICE_ID"),
			CloudAnnualPriceID: os.Getenv("STRIPE_CLOUD_ANNUAL_PRICE_ID"),
			SuccessURL:        baseURL + "/account?session_id={CHECKOUT_SESSION_ID}",
			CancelURL:         baseURL + "/pricing",
		},
		BaseURL:     baseURL,
		EmailClient: emailClient,
	}

	srv := server.New(db, cfg)

	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Background cleanup goroutine
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	defer cleanupCancel()
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if n, err := srv.SessionStore().DeleteExpired(); err != nil {
					log.Printf("cleanup expired sessions: %v", err)
				} else if n > 0 {
					log.Printf("cleaned up %d expired sessions", n)
				}
				srv.RateLimiter().Cleanup()
			case <-cleanupCtx.Done():
				return
			}
		}
	}()

	go func() {
		fmt.Printf("Billing service running at http://localhost:%s\n", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nShutting down...")
	cleanupCancel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
}
