package bootstrap

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/devrix/devrix/internal/layers/communication/capture/transcript"
	"github.com/devrix/devrix/internal/shared/config"
)

// NewTranscriptWriter constructs the transcript writer used by
// CommunicationGateway.ExpireSession to record session_close events.
//
// DM-20260617-008 W1: extracted from NewContextEngine / ContextEngineBuilder.Build
// so the writer is owned by the caller and injected via
// CommunicationGateway's `writer` field (no process-wide global).
//
// dir resolution order:
//  1. ctxCfg.Diagnostics.TranscriptDir
//  2. $DEVRIX_TRANSCRIPT_DIR
//  3. ~/.devrix/transcripts
//
// Returns nil if all sources resolve to empty (caller should pass nil to
// NewCommunicationGateway to disable transcript logging) or if the writer
// fails to initialize (logged as warning).
func NewTranscriptWriter(ctxCfg *config.ContextEngineConfig) *transcript.Writer {
	if ctxCfg == nil {
		return nil
	}
	diagCfg := ctxCfg.Diagnostics.Normalized()
	tdir := diagCfg.TranscriptDir
	if tdir == "" {
		tdir = os.Getenv("DEVRIX_TRANSCRIPT_DIR")
	}
	if tdir == "" {
		if home, herr := os.UserHomeDir(); herr == nil {
			tdir = filepath.Join(home, ".devrix", "transcripts")
		}
	}
	if tdir == "" {
		return nil
	}
	tw, err := transcript.NewWriter(tdir)
	if err != nil {
		slog.Warn("transcript writer init failed", "dir", tdir, "error", err)
		return nil
	}
	slog.Info("transcript writer initialized", "dir", tdir)
	return tw
}