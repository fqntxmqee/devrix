package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/devrix/devrix/internal/layers/observability"
)

func main() {
	cfg := observability.DefaultConfig()
	obs, err := observability.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init observability: %v\n", err)
		os.Exit(1)
	}

	report := obs.CoverageReport(true)
	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal report: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}
