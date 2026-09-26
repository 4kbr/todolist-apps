// Package logger membuat structured logger (slog) untuk seluruh aplikasi.
package logger

import (
	"log/slog"
	"os"
)

// New membuat logger baru dengan level dan environment yang diberikan
func New(level slog.Level, appEnv string) *slog.Logger {
	handlerOpts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if appEnv == "production" {
		// Production: JSON structured logging
		handler = slog.NewJSONHandler(os.Stdout, handlerOpts)
	} else {
		// development: human-readable text logging
		handler = slog.NewTextHandler(os.Stdout, handlerOpts)
	}

	return slog.New(handler)
}
