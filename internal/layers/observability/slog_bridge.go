package observability

import (
	"log/slog"
	"os"

	"github.com/devrix/devrix/internal/layers/observability/logger"
)

// InstallSlogBridge wraps the default slog handler to inject traceId/spanId from context.
func InstallSlogBridge() {
	inner := slog.Default().Handler()
	if inner == nil {
		inner = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	slog.SetDefault(slog.New(logger.NewContextHandler(inner)))
}
