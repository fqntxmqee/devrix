// Command devrix-layer-lint scans internal/layers/ for forbidden D{N}→D{M}
// (N>M) imports. The default output is text; --format=json emits a parseable
// report. --strict exits 1 on any violation, which CI uses as a gate.
//
// Covers: L5-0-0-01  (layer-lint detects reverse D{N}→D{N} imports)
// Covers: L5-0-0-03  (CI gate uses this scanner to block violations)
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/devrix/devrix/internal/lint/layer"
)

func main() {
	root := flag.String("root", "internal/layers", "directory to scan")
	format := flag.String("format", "text", "output format: text|json")
	strict := flag.Bool("strict", false, "exit 1 on any violation")
	flag.Parse()

	pkgs, err := layer.ParseImportGraph(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devrix-layer-lint: failed to walk %s: %v\n", *root, err)
		os.Exit(2)
	}
	matrix := layer.DefaultMatrix()
	violations := layer.ScanPackages(pkgs, matrix)

	switch *format {
	case "text":
		fmt.Print(layer.FormatText(violations))
	case "json":
		fmt.Println(layer.FormatJSON(violations))
	default:
		fmt.Fprintf(os.Stderr, "devrix-layer-lint: unknown format %q (use text or json)\n", *format)
		os.Exit(2)
	}

	if *strict && len(violations) > 0 {
		fmt.Fprintf(os.Stderr, "devrix-layer-lint: %d violation(s) detected (strict mode)\n", len(violations))
		os.Exit(1)
	}
}
