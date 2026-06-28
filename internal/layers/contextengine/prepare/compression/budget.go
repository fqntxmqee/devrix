package compression

import (
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// tokenCounter is the subset of contracts.ITokenCounter used by compression helpers.
// Defined as an alias so compression_steps.go can name it in function signatures
// without dragging the full contracts package into the helper file's public surface.
//
// DM-20260629-002 devrix-d2-dsaft-restructuring PR-2: introduced alongside the
// pipeline.go god-fn split so helpers can declare narrow dependencies.
type tokenCounter = contracts.ITokenCounter

// ShouldCompress returns true if compression should run for the given messages
// and budget. Mirrors the heuristic in RunForSession (lines 82-97 + budget check):
//   - maxMessages overflow, OR
//   - current tokens exceed CompressionTarget.
//
// DM-20260629-002 devrix-d2-dsaft-restructuring PR-2: extracted from pipeline.go
// into budget.go so the pipeline orchestrator only owns flow control.
func (p *Pipeline) ShouldCompress(msgs []types.Message, budget types.TokenBudget) bool {
	if p.maxMessages > 0 && len(msgs) > p.maxMessages {
		return true
	}
	return p.counter.CountMessages(msgs) > budget.CompressionTarget
}

// CountMessages exposes token counting through the pipeline's ITokenCounter.
//
// DM-20260629-002 devrix-d2-dsaft-restructuring PR-2: kept here for backward
// compatibility with callers that don't hold a direct ITokenCounter reference.
func (p *Pipeline) CountMessages(msgs []types.Message) int {
	return p.counter.CountMessages(msgs)
}

// checkBudget validates the final compressed output fits within
// (MaxContextTokens - ReservedOutput). Returns an errors.ContextExceededError
// sentinel when exceeded so callers can map to API-level error codes.
//
// DM-20260629-002 devrix-d2-dsaft-restructuring PR-2: extracted from pipeline.go
// RunForSession's tail (lines 167-170) so the orchestrator stays focused on
// step ordering, not budget arithmetic.
func (p *Pipeline) checkBudget(current []types.Message, budget types.TokenBudget, report *types.CompressionReport) error {
	if p.counter.CountMessages(current) > budget.MaxContextTokens-budget.ReservedOutput {
		report.StepsApplied = append(report.StepsApplied, stepTokenBlock)
		return errors.NewContextExceededError()
	}
	return nil
}

// minKeepForAutocompact computes the minimum number of messages that
// snip() must preserve for autocompact's head+middle+tail strategy to be
// recoverable. Defaults to 4 if autocompact is disabled.
//
// DM-20260629-002 devrix-d2-dsaft-restructuring PR-2: extracted from pipeline.go
// RunForSession's snip step closure.
func (p *Pipeline) minKeepForAutocompact() int {
	const defaultMinKeep = 4
	if !p.autocompactCfg.Enabled {
		return defaultMinKeep
	}
	turns := p.autocompactCfg.PreserveHeadTurns + p.autocompactCfg.PreserveTailTurns + 1
	if turns < 3 {
		turns = 3
	}
	// Each turn needs at least one user message; keep enough messages for head+middle+tail turns.
	minKeep := turns * 2
	if minKeep < defaultMinKeep {
		return defaultMinKeep
	}
	return minKeep
}