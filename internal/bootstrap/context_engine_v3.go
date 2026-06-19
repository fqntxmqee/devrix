package bootstrap

import (
	"log/slog"

	persistmemory "github.com/devrix/devrix/internal/layers/contextengine/persist/memory"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// WireContextV3 builds optional long-term memory (S15-A02 + S17-A03)
// from config and returns a (recaller, store) pair.
//
// P4 split (AC-P4-3): the two interfaces are returned independently so
// the consumer in prepare/memory.Manager can inject them as separate
// fields. The disabled stub satisfies both; the SQLite path returns the
// same *SQLiteLongTerm as both ports.
func WireContextV3(ctxCfg *config.ContextEngineConfig) (contracts.LongTermRecaller, contracts.LongTermStore) {
	if ctxCfg == nil {
		ctxCfg = config.DefaultContextEngineConfig()
	}
	recaller, store, err := persistmemory.NewLongTermFromConfig(ctxCfg.LongTerm)
	if err != nil {
		slog.Warn("longterm memory init failed, using disabled stub", "error", err)
		disabled := persistmemory.NewDisabledLongTerm()
		return disabled, disabled
	}
	return recaller, store
}
