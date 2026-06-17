package redis

import (
	"log/slog"
	"os"
)

func testSlogLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, nil))
}
