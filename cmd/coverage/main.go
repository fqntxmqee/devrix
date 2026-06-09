package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/devrix/devrix/internal/layers/observability/coverage"
)

func main() {
	list := flag.Bool("list", false, "List all coverage reports")
	date := flag.String("date", "", "Show report for specific date (YYYY-MM-DD)")
	trend := flag.Int("trend", 0, "Show coverage trend for last N days (0=disabled)")
	export := flag.String("export", "", "Export report as JSON to file")
	flag.Parse()

	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".devrix", "coverage")

	cli, err := coverage.NewCLI(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	switch {
	case *list:
		if err := cli.ListReports(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case *date != "":
		if err := cli.ShowReport(*date); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case *trend > 0:
		if err := cli.ShowTrend(*trend); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case *export != "":
		if err := cli.ExportJSON(*export, ""); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	default:
		// 默认显示最新报表
		if err := cli.ShowTrend(7); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}
