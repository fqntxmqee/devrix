package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/devrix/devrix/internal/layers/observability/diagnose/coverage"
)

func main() {
	list := flag.Bool("list", false, "List all coverage reports")
	date := flag.String("date", "", "Show report for specific date (YYYY-MM-DD)")
	trend := flag.Int("trend", 0, "Show coverage trend for last N days")
	summary := flag.Bool("summary", false, "Show compact summary by layer")
	flag.Parse()

	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".devrix", "coverage")

	persistence, err := coverage.NewPersistence(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	reporter := coverage.NewReporter(persistence, coverage.Global(), coverage.AllOperations(), 0)

	switch {
	case *list:
		reports, err := persistence.ListDailyReports()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Available coverage reports:")
		for _, r := range reports {
			d := r[9 : len(r)-5]
			rep, err := persistence.LoadDailyReport(d)
			if err != nil {
				continue
			}
			fmt.Printf("  %s | coverage: %5.1f%% | hit: %d/%d | zero: %d\n",
				d, rep.CoverageRatio*100, rep.OperationsHit,
				rep.OperationsTotal, rep.OperationsZero)
		}

	case *date != "":
		if *summary {
			if err := reporter.PrintSummary(*date); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			if err := reporter.PrintDailyReport(*date); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}

	case *trend > 0:
		if err := reporter.PrintTrend(*trend); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	default:
		report, err := persistence.GetLatestReport()
		if err != nil {
			fmt.Fprintf(os.Stderr, "No coverage reports found: %v\n", err)
			os.Exit(1)
		}
		if err := reporter.PrintDailyReport(report.Date); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}
