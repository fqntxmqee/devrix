package observability

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/observability/tracer"
)

const defaultLLMLogDir = "~/.devrix/logs/llm"

const (
	defaultMsgTruncate  = 500
	defaultRespTruncate = 2000
	spanPreviewTruncate = 16384
)

// LLMLogSettings controls LLM content capture for tracing and local files.
type LLMLogSettings struct {
	LogContent bool
	LogDir     string
}

var (
	llmLogMu       sync.RWMutex
	llmLogSettings = LLMLogSettings{LogDir: defaultLLMLogDir}
	llmLogFileMu   sync.Mutex
)

// ConfigureLLMLogging applies observability.llm settings at process startup.
func ConfigureLLMLogging(settings LLMLogSettings) {
	llmLogMu.Lock()
	defer llmLogMu.Unlock()

	llmLogSettings = settings
	if strings.TrimSpace(llmLogSettings.LogDir) == "" {
		llmLogSettings.LogDir = defaultLLMLogDir
	}
	llmLogSettings.LogDir = expandHomePath(llmLogSettings.LogDir)
}

func currentLLMLogSettings() LLMLogSettings {
	llmLogMu.RLock()
	defer llmLogMu.RUnlock()
	return llmLogSettings
}

// CurrentLLMLogSettings returns the active LLM logging configuration.
func CurrentLLMLogSettings() LLMLogSettings {
	return currentLLMLogSettings()
}

// LLMLogContentEnabled reports whether full LLM payload capture is enabled.
func LLMLogContentEnabled() bool {
	return currentLLMLogSettings().LogContent
}

// RecordLLMSpanPayload writes LLM JSON to span tags, span events, and optional local file.
func RecordLLMSpanPayload(
	span tracer.Span,
	sessionID, phase, eventName, jsonAttrKey, jsonValue string,
	iteration int,
	model string,
	extra ...tracer.Attribute,
) {
	settings := currentLLMLogSettings()
	if settings.LogContent && jsonValue != "" {
		appendLLMLogRaw(settings.LogDir, sessionID, phase, iteration, model, json.RawMessage(jsonValue))
	}

	if span == nil {
		return
	}

	attrs := append([]tracer.Attribute{}, extra...)
	attrs = append(attrs, tracer.Attribute{Key: jsonAttrKey, Value: jsonValue})
	span.SetAttributes(attrs...)
	span.AddEvent(eventName, tracer.WithEventAttributes(attrs...))
}

// AppendLLMLogFile writes a structured record to the local JSONL log.
func AppendLLMLogFile(logDir, sessionID, phase string, iter int, model string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		slog.Warn("llm log: marshal payload failed", "error", err)
		return
	}
	appendLLMLogRaw(logDir, sessionID, phase, iter, model, data)
}

func appendLLMLogRaw(logDir, sessionID, phase string, iter int, model string, data json.RawMessage) {
	record := map[string]interface{}{
		"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		"session_id": sessionID,
		"phase":      phase,
		"iteration":  iter,
		"model":      model,
		"data":       json.RawMessage(data),
	}
	line, err := json.Marshal(record)
	if err != nil {
		slog.Warn("llm log: marshal record failed", "error", err)
		return
	}

	llmLogFileMu.Lock()
	defer llmLogFileMu.Unlock()

	if err := os.MkdirAll(logDir, 0o755); err != nil {
		slog.Warn("llm log: mkdir failed", "dir", logDir, "error", err)
		return
	}
	path := filepath.Join(logDir, sanitizeSessionFilename(sessionID)+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Warn("llm log: open file failed", "path", path, "error", err)
		return
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		slog.Warn("llm log: write failed", "path", path, "error", err)
	}
}

func expandHomePath(path string) string {
	if path == "" {
		return path
	}
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func sanitizeSessionFilename(sessionID string) string {
	if strings.TrimSpace(sessionID) == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range sessionID {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// TruncateForSpan limits payload size when full logging is disabled.
func TruncateForSpan(s string, full bool) string {
	if full || len(s) <= spanPreviewTruncate {
		return s
	}
	if len(s) <= defaultMsgTruncate {
		return s
	}
	return s[:defaultMsgTruncate] + "..."
}

// TruncateResponseForSpan limits response preview size.
func TruncateResponseForSpan(s string, full bool) string {
	if full || len(s) <= spanPreviewTruncate {
		return s
	}
	limit := defaultRespTruncate
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
