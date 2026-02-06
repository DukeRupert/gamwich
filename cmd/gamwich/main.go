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
	"github.com/dukerupert/gamwich/internal/server"
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

	weatherCfg := weather.Config{
		Latitude:        os.Getenv("GAMWICH_WEATHER_LAT"),
		Longitude:       os.Getenv("GAMWICH_WEATHER_LON"),
		TemperatureUnit: os.Getenv("GAMWICH_WEATHER_UNITS"),
	}
	if weatherCfg.TemperatureUnit == "" {
		weatherCfg.TemperatureUnit = "fahrenheit"
	}
	weatherSvc := weather.NewService(weatherCfg)

	srv := server.New(db, weatherSvc)

	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      srv.Router(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
}
