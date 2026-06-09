package coverage

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Reporter periodically generates and persists coverage reports
type Reporter struct {
	persistence *Persistence
	counter     *Counter
	registry    []OperationMeta
	interval    time.Duration
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// NewReporter creates a new coverage reporter
func NewReporter(persistence *Persistence, counter *Counter, registry []OperationMeta, interval time.Duration) *Reporter {
	return &Reporter{
		persistence: persistence,
		counter:    counter,
		registry:   registry,
		interval:   interval,
		stopCh:    make(chan struct{}),
	}
}

// Start begins the periodic reporting
func (r *Reporter) Start(ctx context.Context) {
	r.wg.Add(1)
	go r.run(ctx)
	slog.Info("coverage reporter started", "interval", r.interval.String())
}

// Stop stops the periodic reporting
func (r *Reporter) Stop() {
	close(r.stopCh)
	r.wg.Wait()
	slog.Info("coverage reporter stopped")
}

func (r *Reporter) run(ctx context.Context) {
	defer r.wg.Done()
	
	// 计算到下一个整点的时间
	now := time.Now()
	nextHour := now.Truncate(time.Hour).Add(time.Hour)
	timer := time.NewTimer(time.Until(nextHour))
	defer timer.Stop()
	
	// 等待到整点
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		// 继续到下面的循环
	case <-r.stopCh:
		return
	}
	
	// 定时器，每小时检查一次是否需要生成日报
	hourlyTick := time.NewTicker(time.Hour)
	defer hourlyTick.Stop()
	
	// 记录上次生成日报的日期
	lastReportDate := now.Format("2006-01-02")
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case <-hourlyTick.C:
			currentDate := time.Now().Format("2006-01-02")
			if currentDate != lastReportDate {
				// 新的一天，生成日报
				report, err := r.persistence.GenerateDailyReport(r.counter, r.registry)
				if err != nil {
					slog.Error("failed to generate daily coverage report", "error", err)
				} else {
					slog.Info("daily coverage report generated",
						"date", report.Date,
						"hit", report.OperationsHit,
						"total", report.OperationsTotal,
						"coverage", fmt.Sprintf("%.1f%%", report.CoverageRatio*100),
						"zero_hit", report.OperationsZero,
					)
				}
				lastReportDate = currentDate
			}
		}
	}
}

// GenerateNow generates a report immediately
func (r *Reporter) GenerateNow() (*DailyReport, error) {
	return r.persistence.GenerateDailyReport(r.counter, r.registry)
}

// GetTrend returns coverage trend for the specified number of days
func (r *Reporter) GetTrend(days int) ([]*DailyReport, error) {
	return r.persistence.GetTrend(days)
}

// PrintTrend prints a formatted coverage trend report
func (r *Reporter) PrintTrend(days int) error {
	reports, err := r.GetTrend(days)
	if err != nil {
		return err
	}
	
	if len(reports) == 0 {
		fmt.Println("No coverage data available")
		return nil
	}
	
	fmt.Println("\n========== Coverage Trend ==========")
	fmt.Printf("Period: %s → %s\n\n", reports[0].Date, reports[len(reports)-1].Date)
	
	for _, r := range reports {
		barLen := int(r.CoverageRatio * 40)
		bar := ""
		for i := 0; i < barLen; i++ {
			bar += "█"
		}
		for i := barLen; i < 40; i++ {
			bar += "░"
		}
		
		fmt.Printf("%s |%s| %5.1f%% hit:%d zero:%d\n", 
			r.Date, bar, r.CoverageRatio*100, r.OperationsHit, r.OperationsZero)
	}
	
	fmt.Println("===================================")
	return nil
}

// PrintDailyReport prints a formatted daily report
func (r *Reporter) PrintDailyReport(date string) error {
	report, err := r.persistence.LoadDailyReport(date)
	if err != nil {
		return err
	}
	
	fmt.Printf("\n========== Coverage Report: %s ==========\n", report.Date)
	fmt.Printf("Generated: %s\n", report.GeneratedAt.Format("15:04:05"))
	fmt.Printf("Coverage: %.1f%% (%d/%d operations hit)\n", 
		report.CoverageRatio*100, report.OperationsHit, report.OperationsTotal)
	fmt.Printf("Unknown operations: %d\n", report.UnknownHits)
	
	if report.OperationsZero > 0 {
		fmt.Printf("\n--- Zero-hit operations (%d) ---\n", report.OperationsZero)
		// 按 layer 分组
		byLayer := make(map[string][]ZeroHitEntry)
		for _, op := range report.ZeroHitOps {
			byLayer[op.Layer] = append(byLayer[op.Layer], op)
		}
		
		for layer, ops := range byLayer {
			fmt.Printf("\n[%s]\n", layer)
			for _, op := range ops {
				fmt.Printf("  • %s (since %s)\n", op.Operation, op.SinceVersion)
			}
		}
	}
	
	fmt.Println("\n===================================")
	return nil
}
