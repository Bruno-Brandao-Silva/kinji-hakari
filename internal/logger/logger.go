package logger

import (
	"log/slog"
	"os"
)

// Init installs a text logger on stdout. LOG_LEVEL accepts debug, info, warn or error (default info).
func Init() {
	level := slog.LevelInfo
	if value := os.Getenv("LOG_LEVEL"); value != "" {
		if err := level.UnmarshalText([]byte(value)); err != nil {
			level = slog.LevelInfo
			defer slog.Warn("LOG_LEVEL inválido, usando info", "value", value)
		}
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}
