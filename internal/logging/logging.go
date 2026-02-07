package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Setup creates a configured *slog.Logger, sets it as the default, and returns it.
// The level parameter accepts: "debug", "info", "warn", "error" (case-insensitive).
// Defaults to info if the level string is unrecognized.
func Setup(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: lvl,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
