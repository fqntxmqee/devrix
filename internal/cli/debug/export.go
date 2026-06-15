package debug

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/devrix/devrix/internal/layers/observability/diagnose/incident"
)

// RunExport executes `debug export` with the given args (after `debug export`).
func RunExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	sessionID := fs.String("session", "", "session ID to export (required)")
	output := fs.String("output", "", "output file path (default: stdout)")
	format := fs.String("format", "json", "export format (json)")
	llmLogDir := fs.String("llm-log-dir", "", "LLM JSONL directory (default: ~/.devrix/logs/llm)")
	coverageDir := fs.String("coverage-dir", "", "coverage snapshots directory (optional)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *sessionID == "" {
		return fmt.Errorf("--session is required")
	}
	if *format != "json" {
		return fmt.Errorf("unsupported format %q", *format)
	}

	bundle, err := incident.BuildBundle(*sessionID, incident.ExportOptions{
		LLMLogDir:   *llmLogDir,
		CoverageDir: *coverageDir,
	})
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	data, err := incident.MarshalJSON(bundle)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	if *output == "" {
		_, err = os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(*output, data, 0o644)
}

// Run dispatches debug subcommands.
func Run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: devrix debug export --session <id>")
	}
	switch args[0] {
	case "export":
		return RunExport(args[1:])
	default:
		return fmt.Errorf("unknown debug command %q", args[0])
	}
}
