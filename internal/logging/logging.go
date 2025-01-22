package logging

import (
	"log/slog"
	"os"
)

func CreateLogger(module string) *slog.Logger {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelInfo,
	}))

	return logger.With("module", module)
}
