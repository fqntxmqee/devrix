package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/devrix/devrix/internal/layers/observability/incident"
)

func main() {
	sessionID := flag.String("session", "", "session ID to export (required)")
	output := flag.String("output", "", "output file path (default: stdout)")
	format := flag.String("format", "json", "export format (json)")
	llmLogDir := flag.String("llm-log-dir", "", "LLM JSONL directory (default: ~/.devrix/logs/llm)")
	coverageDir := flag.String("coverage-dir", "", "coverage snapshots directory (optional)")
	flag.Parse()

	if *sessionID == "" {
		fmt.Fprintln(os.Stderr, "error: --session is required")
		os.Exit(2)
	}
	if *format != "json" {
		fmt.Fprintf(os.Stderr, "error: unsupported format %q\n", *format)
		os.Exit(2)
	}

	bundle, err := incident.BuildBundle(*sessionID, incident.ExportOptions{
		LLMLogDir:   *llmLogDir,
		CoverageDir: *coverageDir,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "export failed: %v\n", err)
		os.Exit(1)
	}

	data, err := incident.MarshalJSON(bundle)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal failed: %v\n", err)
		os.Exit(1)
	}

	if *output == "" {
		fmt.Print(string(data))
		return
	}
	if err := os.WriteFile(*output, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write failed: %v\n", err)
		os.Exit(1)
	}
}
