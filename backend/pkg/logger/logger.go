// Package logger configures a slog-based structured logger.
package logger

import (
	"log/slog"
	"os"
)

// New creates a new *slog.Logger.
//
//   - level:  "debug" | "info" | "warn" | "error"
//   - format: "json" (production) | "text" (development)
func New(level, format string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
