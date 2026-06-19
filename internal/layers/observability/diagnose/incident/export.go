package incident

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/observability/diagnose/coverage"
)

const schemaVersion = "1.0"

// Bundle is the session incident export schema v1.
type Bundle struct {
	SchemaVersion string            `json:"schema_version"`
	SessionID     string            `json:"session_id"`
	ExportedAt    string            `json:"exported_at"`
	Trace         *TraceSection     `json:"trace,omitempty"`
	LLMRounds     []json.RawMessage `json:"llm_rounds"`
	Errors        []string          `json:"errors,omitempty"`
	CoverageHits  []CoverageHit     `json:"coverage_hits,omitempty"`
	EvalScores    map[string]string `json:"eval_scores,omitempty"`
	PromptVersions map[string]string `json:"prompt_versions,omitempty"`
}

// TraceSection holds optional trace tree data (populated when spans are available).
type TraceSection struct {
	TraceID string      `json:"trace_id,omitempty"`
	Spans   []SpanEntry `json:"spans,omitempty"`
}

// SpanEntry is a simplified span for incident bundles.
type SpanEntry struct {
	Name         string                 `json:"name"`
	SpanID       string                 `json:"span_id,omitempty"`
	ParentSpanID string                 `json:"parent_span_id,omitempty"`
	StartTime    string                 `json:"start_time,omitempty"`
	DurationMs   int64                  `json:"duration_ms,omitempty"`
	Attributes   map[string]interface{} `json:"attributes,omitempty"`
}

// CoverageHit summarizes runtime coverage for an operation.
type CoverageHit struct {
	Operation string `json:"operation"`
	Layer     string `json:"layer"`
	HitCount  uint64 `json:"hit_count"`
	LastHit   string `json:"last_hit,omitempty"`
}

// ExportOptions configures bundle assembly.
type ExportOptions struct {
	LLMLogDir     string
	CoverageDir   string
	MemorySpans   func() []SpanEntry
	EvalScores    map[string]string
	PromptVersions map[string]string
}

// BuildBundle assembles an incident bundle for the given session.
func BuildBundle(sessionID string, opts ExportOptions) (*Bundle, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	logDir := opts.LLMLogDir
	if logDir == "" {
		logDir = CurrentLLMLogSettings().LogDir
	}

	rounds, traceID, err := readLLMRounds(logDir, sessionID)
	if err != nil {
		return nil, err
	}

	bundle := &Bundle{
		SchemaVersion:  schemaVersion,
		SessionID:      sessionID,
		ExportedAt:     time.Now().UTC().Format(time.RFC3339),
		LLMRounds:      rounds,
		EvalScores:     opts.EvalScores,
		PromptVersions: opts.PromptVersions,
	}

	if traceID != "" {
		bundle.Trace = &TraceSection{TraceID: traceID}
	}
	if opts.MemorySpans != nil {
		spans := opts.MemorySpans()
		if len(spans) > 0 {
			if bundle.Trace == nil {
				bundle.Trace = &TraceSection{}
			}
			bundle.Trace.Spans = spans
			if bundle.Trace.TraceID == "" && len(spans) > 0 {
				// trace id may be embedded in attributes later; keep optional
			}
		}
	}

	if opts.CoverageDir != "" {
		hits, covErr := readCoverageHits(opts.CoverageDir)
		if covErr != nil {
			bundle.Errors = append(bundle.Errors, covErr.Error())
		} else {
			bundle.CoverageHits = hits
		}
	}

	return bundle, nil
}

// MarshalJSON returns indented JSON for the bundle.
func MarshalJSON(bundle *Bundle) ([]byte, error) {
	return json.MarshalIndent(bundle, "", "  ")
}

func readLLMRounds(logDir, sessionID string) ([]json.RawMessage, string, error) {
	path := filepath.Join(logDir, sanitizeSessionFilename(sessionID)+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("open llm log: %w", err)
	}
	defer f.Close()

	var rounds []json.RawMessage
	var traceID string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var header struct {
			TraceID string `json:"trace_id"`
		}
		if err := json.Unmarshal(line, &header); err == nil && header.TraceID != "" && traceID == "" {
			traceID = header.TraceID
		}
		rounds = append(rounds, json.RawMessage(line))
	}
	if err := scanner.Err(); err != nil {
		return nil, "", fmt.Errorf("read llm log: %w", err)
	}
	return rounds, traceID, nil
}

func readCoverageHits(dir string) ([]CoverageHit, error) {
	p, err := coverage.NewPersistence(dir)
	if err != nil {
		return nil, err
	}
	report, err := p.GetLatestReport()
	if err != nil {
		if strings.Contains(err.Error(), "no reports found") {
			return nil, nil
		}
		return nil, err
	}
	if report == nil {
		return nil, nil
	}
	var hits []CoverageHit
	for op, count := range report.Hits {
		if count == 0 {
			continue
		}
		hits = append(hits, CoverageHit{
			Operation: op,
			HitCount:  count,
		})
	}
	return hits, nil
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
