package bootstrap

import (
	"log/slog"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/shared/config"
)

// WireContextV3 builds optional LongTerm memory from config.
func WireContextV3(
	ctxCfg *config.ContextEngineConfig,
) memory.ILongTermMemory {
	if ctxCfg == nil {
		ctxCfg = config.DefaultContextEngineConfig()
	}

	longTerm, err := memory.NewLongTermFromConfig(ctxCfg.LongTerm)
	if err != nil {
		slog.Warn("longterm memory init failed, using disabled stub", "error", err)
		longTerm = memory.NewDisabledLongTermMemory()
	}
	return longTerm
}
