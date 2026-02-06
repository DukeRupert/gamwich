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

	"github.com/dukerupert/gamwich/internal/database"
	"github.com/dukerupert/gamwich/internal/email"
	"github.com/dukerupert/gamwich/internal/server"
	"github.com/dukerupert/gamwich/internal/store"
	"github.com/dukerupert/gamwich/internal/weather"
)

func main() {
	port := os.Getenv("GAMWICH_PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("GAMWICH_DB_PATH")
	if dbPath == "" {
		dbPath = "gamwich.db"
	}

	db, err := database.Open(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Read weather config: DB values take priority, env vars as fallback
	weatherCfg := weather.Config{
		Latitude:        os.Getenv("GAMWICH_WEATHER_LAT"),
		Longitude:       os.Getenv("GAMWICH_WEATHER_LON"),
		TemperatureUnit: os.Getenv("GAMWICH_WEATHER_UNITS"),
	}
	settingsStore := store.NewSettingsStore(db)
	if dbWeather, err := settingsStore.GetWeatherSettings(); err == nil {
		if v := dbWeather["weather_latitude"]; v != "" {
			weatherCfg.Latitude = v
		}
		if v := dbWeather["weather_longitude"]; v != "" {
			weatherCfg.Longitude = v
		}
		if v := dbWeather["weather_units"]; v != "" {
			weatherCfg.TemperatureUnit = v
		}
	}
	if weatherCfg.TemperatureUnit == "" {
		weatherCfg.TemperatureUnit = "fahrenheit"
	}
	weatherSvc := weather.NewService(weatherCfg)

	// Email config
	postmarkToken := os.Getenv("GAMWICH_POSTMARK_TOKEN")
	fromEmail := os.Getenv("GAMWICH_FROM_EMAIL")
	baseURL := os.Getenv("GAMWICH_BASE_URL")
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%s", port)
	}
	emailClient := email.NewClient(postmarkToken, fromEmail, baseURL)

	srv := server.New(db, weatherSvc, emailClient, baseURL)

	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		// No ReadTimeout/WriteTimeout — WebSocket connections are long-lived
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
				if n, err := srv.MagicLinkStore().DeleteExpired(); err != nil {
					log.Printf("cleanup expired magic links: %v", err)
				} else if n > 0 {
					log.Printf("cleaned up %d expired magic links", n)
				}
				srv.RateLimiter().Cleanup()
			case <-cleanupCtx.Done():
				return
			}
		}
	}()

	go func() {
		fmt.Printf("Gamwich running at http://localhost:%s\n", port)
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
