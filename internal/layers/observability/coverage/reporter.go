package coverage

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
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

// OpInfo holds operation info for display
type OpInfo struct {
	Operation   string
	Layer       string
	Component   string
	IsHit       bool
	Hits        uint64
	SinceVersion string
}

// ByLayer sorts operations by layer and component
type ByLayer []OpInfo

func (a ByLayer) Len() int      { return len(a) }
func (a ByLayer) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByLayer) Less(i, j int) bool {
	if a[i].Layer != a[j].Layer {
		return a[i].Layer < a[j].Layer
	}
	if a[i].Component != a[j].Component {
		return a[i].Component < a[j].Component
	}
	return a[i].Operation < a[j].Operation
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
	
	fmt.Println("\n========== Coverage Trend (Jaeger Operations) ==========")
	fmt.Printf("Period: %s → %s\n\n", reports[0].Date, reports[len(reports)-1].Date)
	
	for _, r := range reports {
		barLen := int(r.CoverageRatio * 40)
		bar := strings.Repeat("█", barLen) + strings.Repeat("░", 40-barLen)
		fmt.Printf("%s |%s| %5.1f%% hit:%d zero:%d\n", 
			r.Date, bar, r.CoverageRatio*100, r.OperationsHit, r.OperationsZero)
	}
	
	fmt.Println("====================================================")
	return nil
}

// PrintDailyReport prints a detailed report grouped by Jaeger layer/component
func (r *Reporter) PrintDailyReport(date string) error {
	report, err := r.persistence.LoadDailyReport(date)
	if err != nil {
		return err
	}
	
	// 构建 hit 集合
	hits := make(map[string]uint64)
	for op, count := range report.Hits {
		hits[op] = count
	}
	
	// 构建 operation 信息列表
	var ops []OpInfo
	for _, meta := range r.registry {
		info := OpInfo{
			Operation:   meta.Name,
			Layer:       meta.Layer,
			Component:   meta.Component,
			SinceVersion: meta.SinceVersion,
			IsHit:       hits[meta.Name] > 0,
			Hits:        hits[meta.Name],
		}
		ops = append(ops, info)
	}
	
	// 按 layer.component.operation 排序
	sort.Sort(ByLayer(ops))
	
	// 计算统计
	hitByLayer := make(map[string]int)
	zeroByLayer := make(map[string]int)
	totalByLayer := make(map[string]int)
	
	for _, op := range ops {
		key := op.Layer + "." + op.Component
		totalByLayer[key]++
		if op.IsHit {
			hitByLayer[key]++
		} else {
			zeroByLayer[key]++
		}
	}
	
	// 输出报告
	fmt.Printf("\n========== Coverage Report: %s ==========\n", report.Date)
	fmt.Printf("Generated: %s\n", report.GeneratedAt.Format("15:04:05"))
	fmt.Printf("Coverage: %.1f%% (%d/%d operations hit)\n", 
		report.CoverageRatio*100, report.OperationsHit, report.OperationsTotal)
	fmt.Printf("Unknown operations: %d\n\n", report.UnknownHits)
	
	// 按层分组输出
	currentLayer := ""
	currentComponent := ""
	
	for _, op := range ops {
		// Layer 变化
		if op.Layer != currentLayer {
			currentLayer = op.Layer
			currentComponent = ""
			fmt.Printf("\n┌─ %s ──────────────────────────────────────\n", strings.ToUpper(currentLayer))
			fmt.Printf("│  Layer: %s\n", currentLayer)
		}
		
		// Component 变化
		if op.Component != currentComponent {
			currentComponent = op.Component
			key := currentLayer + "." + currentComponent
			hit := hitByLayer[key]
			total := totalByLayer[key]
			fmt.Printf("│\n│  ├─ %s (%d/%d hit)\n", currentComponent, hit, total)
		}
		
		// Operation 行
		status := "○"
		hits := uint64(0)
		if op.IsHit {
			status = "●"
			hits = op.Hits
		}
		
		fmt.Printf("│  │   %s %s [%s] %d hits\n", status, op.Operation, op.SinceVersion, hits)
	}
	
	fmt.Println("│")
	fmt.Println("└────────────────────────────────────────────────")
	fmt.Println("\n====================================================")
	
	return nil
}

// PrintSummary prints a compact summary grouped by layer
func (r *Reporter) PrintSummary(date string) error {
	report, err := r.persistence.LoadDailyReport(date)
	if err != nil {
		return err
	}
	
	hits := make(map[string]uint64)
	for op, count := range report.Hits {
		hits[op] = count
	}
	
	// 按 layer 统计
	type layerStats struct{ hit, total int }
	byLayer := make(map[string]layerStats)
	
	for _, meta := range r.registry {
		s := byLayer[meta.Layer]
		s.total++
		if hits[meta.Name] > 0 {
			s.hit++
		}
		byLayer[meta.Layer] = s
	}
	
	fmt.Printf("\n========== Coverage Summary: %s ==========\n", report.Date)
	fmt.Printf("Total: %.1f%% (%d/%d operations)\n\n", 
		report.CoverageRatio*100, report.OperationsHit, report.OperationsTotal)
	
	// 按 hit 率排序
	type layerStat struct {
		name  string
		hit   int
		total int
	}
	var stats []layerStat
	for name, s := range byLayer {
		stats = append(stats, layerStat{name, s.hit, s.total})
	}
	sort.Slice(stats, func(i, j int) bool {
		ri := float64(stats[i].hit) / float64(stats[i].total)
		rj := float64(stats[j].hit) / float64(stats[j].total)
		return ri > rj
	})
	
	for _, s := range stats {
		ratio := float64(s.hit) / float64(s.total)
		barLen := int(ratio * 20)
		bar := strings.Repeat("█", barLen) + strings.Repeat("░", 20-barLen)
		fmt.Printf("%-15s |%s| %4.1f%% (%d/%d)\n", s.name, bar, ratio*100, s.hit, s.total)
	}
	
	fmt.Println("\n===================================")
	return nil
}
