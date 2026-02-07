package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dukerupert/gamwich/internal/backup"
	"github.com/dukerupert/gamwich/internal/database"
	"github.com/dukerupert/gamwich/internal/email"
	"github.com/dukerupert/gamwich/internal/license"
	"github.com/dukerupert/gamwich/internal/logging"
	"github.com/dukerupert/gamwich/internal/push"
	"github.com/dukerupert/gamwich/internal/server"
	"github.com/dukerupert/gamwich/internal/store"
	"github.com/dukerupert/gamwich/internal/weather"
)

func main() {
	logger := logging.Setup(os.Getenv("GAMWICH_LOG_LEVEL"))

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
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
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

	// Email config: app-level settings from env vars only
	postmarkToken := os.Getenv("GAMWICH_POSTMARK_TOKEN")
	fromEmail := os.Getenv("GAMWICH_FROM_EMAIL")
	baseURL := os.Getenv("GAMWICH_BASE_URL")
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%s", port)
	}
	emailClient := email.NewClient(postmarkToken, fromEmail, baseURL)

	// License client: DB value takes priority, env var as fallback
	licenseKey := os.Getenv("GAMWICH_LICENSE_KEY")
	if dbKey, err := settingsStore.Get("license_key"); err == nil && dbKey != "" {
		licenseKey = dbKey
	}
	licenseClient := license.NewClient(license.Config{
		Key:           licenseKey,
		ValidationURL: os.Getenv("GAMWICH_LICENSE_URL"),
	})

	// Backup S3 config: DB values take priority, env vars as fallback
	backupCfg := backup.Config{
		DBPath: dbPath,
		S3: backup.S3Config{
			Endpoint:  os.Getenv("GAMWICH_BACKUP_S3_ENDPOINT"),
			Bucket:    os.Getenv("GAMWICH_BACKUP_S3_BUCKET"),
			Region:    os.Getenv("GAMWICH_BACKUP_S3_REGION"),
			AccessKey: os.Getenv("GAMWICH_BACKUP_S3_ACCESS_KEY"),
			SecretKey: os.Getenv("GAMWICH_BACKUP_S3_SECRET_KEY"),
		},
	}
	if dbS3, err := settingsStore.GetS3Settings(); err == nil {
		if v := dbS3["backup_s3_endpoint"]; v != "" {
			backupCfg.S3.Endpoint = v
		}
		if v := dbS3["backup_s3_bucket"]; v != "" {
			backupCfg.S3.Bucket = v
		}
		if v := dbS3["backup_s3_region"]; v != "" {
			backupCfg.S3.Region = v
		}
		if v := dbS3["backup_s3_access_key"]; v != "" {
			backupCfg.S3.AccessKey = v
		}
		if v := dbS3["backup_s3_secret_key"]; v != "" {
			backupCfg.S3.SecretKey = v
		}
	}

	// Push notification config: DB values take priority, auto-generate + persist if empty
	pushCfg := push.Config{
		VAPIDPublicKey:  os.Getenv("GAMWICH_VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey: os.Getenv("GAMWICH_VAPID_PRIVATE_KEY"),
	}
	if dbVAPID, err := settingsStore.GetVAPIDSettings(); err == nil {
		if v := dbVAPID["vapid_public_key"]; v != "" {
			pushCfg.VAPIDPublicKey = v
		}
		if v := dbVAPID["vapid_private_key"]; v != "" {
			pushCfg.VAPIDPrivateKey = v
		}
	}
	if pushCfg.VAPIDPublicKey == "" || pushCfg.VAPIDPrivateKey == "" {
		pub, priv, err := push.GenerateVAPIDKeys()
		if err != nil {
			slog.Error("generate VAPID keys", "error", err)
		} else {
			pushCfg.VAPIDPublicKey = pub
			pushCfg.VAPIDPrivateKey = priv
			// Persist to DB so keys survive restarts without env vars
			settingsStore.Set("vapid_public_key", pub)
			settingsStore.Set("vapid_private_key", priv)
			slog.Info("auto-generated and persisted VAPID keys")
		}
	}

	srv := server.New(db, weatherSvc, emailClient, baseURL, licenseClient, port, backupCfg, pushCfg, logger)

	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		// No ReadTimeout/WriteTimeout — WebSocket connections are long-lived
	}

	// Start license validation
	licenseCtx, licenseCancel := context.WithCancel(context.Background())
	defer licenseCancel()
	licenseClient.Start(licenseCtx)

	// Start tunnel if enabled and licensed
	tunnelCtx, tunnelCancel := context.WithCancel(context.Background())
	defer tunnelCancel()
	if licenseClient.HasFeature("tunnel") {
		if err := srv.TunnelManager().Start(tunnelCtx); err != nil {
			slog.Error("tunnel start", "error", err)
		}
	}

	// Start backup manager if configured and licensed
	backupCtx, backupCancel := context.WithCancel(context.Background())
	defer backupCancel()
	if licenseClient.HasFeature("backup") {
		srv.BackupManager().Start(backupCtx)
	}

	// Start push scheduler if licensed
	pushCtx, pushCancel := context.WithCancel(context.Background())
	defer pushCancel()
	if licenseClient.HasFeature("push_notifications") && srv.PushScheduler() != nil {
		srv.PushScheduler().Start(pushCtx)
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
					slog.Error("cleanup expired sessions", "error", err)
				} else if n > 0 {
					slog.Info("cleaned up expired sessions", "count", n)
				}
				if n, err := srv.MagicLinkStore().DeleteExpired(); err != nil {
					slog.Error("cleanup expired magic links", "error", err)
				} else if n > 0 {
					slog.Info("cleaned up expired magic links", "count", n)
				}
				srv.RateLimiter().Cleanup()
				// Clean up old sent_notifications (older than 7 days)
				if ps := srv.PushStore(); ps != nil {
					if err := ps.CleanupSent(time.Now().UTC().AddDate(0, 0, -7)); err != nil {
						slog.Error("cleanup sent notifications", "error", err)
					}
				}
			case <-cleanupCtx.Done():
				return
			}
		}
	}()

	go func() {
		slog.Info("server starting", "addr", ":"+port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")
	backupCancel()
	srv.BackupManager().Stop()
	tunnelCancel()
	srv.TunnelManager().Stop()
	pushCancel()
	if srv.PushScheduler() != nil {
		srv.PushScheduler().Stop()
	}
	cleanupCancel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "error", err)
		os.Exit(1)
	}

}
