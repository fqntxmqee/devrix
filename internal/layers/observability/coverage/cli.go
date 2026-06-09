package coverage

import (
	"fmt"
	"os"
)

// CLI provides command-line interface for coverage reports
type CLI struct {
	persistence *Persistence
}

// NewCLI creates a new coverage CLI
func NewCLI(dir string) (*CLI, error) {
	persistence, err := NewPersistence(dir)
	if err != nil {
		return nil, err
	}
	return &CLI{persistence: persistence}, nil
}

// ListReports lists all available reports
func (c *CLI) ListReports() error {
	reports, err := c.persistence.ListDailyReports()
	if err != nil {
		return err
	}

	if len(reports) == 0 {
		fmt.Println("No coverage reports available")
		return nil
	}

	fmt.Println("Available coverage reports:")
	for _, r := range reports {
		date := r[9 : len(r)-5]
		report, err := c.persistence.LoadDailyReport(date)
		if err != nil {
			continue
		}
		fmt.Printf("  %s | coverage: %5.1f%% | hit: %d/%d | zero: %d\n",
			date, report.CoverageRatio*100, report.OperationsHit,
			report.OperationsTotal, report.OperationsZero)
	}
	return nil
}

// ShowReport shows a specific day's report
func (c *CLI) ShowReport(date string) error {
	reporter := NewReporter(c.persistence, nil, nil, 0)
	return reporter.PrintDailyReport(date)
}

// ShowSummary shows a compact summary
func (c *CLI) ShowSummary(date string) error {
	reporter := NewReporter(c.persistence, nil, nil, 0)
	return reporter.PrintSummary(date)
}

// ShowTrend shows coverage trend
func (c *CLI) ShowTrend(days int) error {
	reporter := NewReporter(c.persistence, nil, nil, 0)
	return reporter.PrintTrend(days)
}

// ExportJSON exports a report as JSON
func (c *CLI) ExportJSON(date, output string) error {
	report, err := c.persistence.LoadDailyReport(date)
	if err != nil {
		return err
	}

	if output == "" || output == "-" {
		fmt.Println(report.JSON())
		return nil
	}

	return os.WriteFile(output, []byte(report.JSON()), 0644)
}
