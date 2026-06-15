package coverage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Persistence handles coverage data persistence to disk
type Persistence struct {
	dir      string
	mu       sync.RWMutex
	lastSnap map[string]uint64
}

// NewPersistence creates a new persistence manager
func NewPersistence(dir string) (*Persistence, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create coverage dir: %w", err)
	}
	return &Persistence{
		dir:      dir,
		lastSnap: make(map[string]uint64),
	}, nil
}

// DailyReport represents a daily coverage report
type DailyReport struct {
	Date             string            `json:"date"`
	GeneratedAt      time.Time         `json:"generated_at"`
	OperationsTotal  int               `json:"operations_total"`
	OperationsHit    int               `json:"operations_hit"`
	OperationsZero   int               `json:"operations_zero"`
	CoverageRatio   float64           `json:"coverage_ratio"`
	ZeroHitOps      []ZeroHitEntry    `json:"zero_hit_operations,omitempty"`
	UnknownHits      uint64           `json:"unknown_hits"`
	Hits            map[string]uint64 `json:"hits,omitempty"`
}

// JSON returns JSON representation
func (r *DailyReport) JSON() string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}

// PersistCounter persists the current counter state to disk
func (p *Persistence) PersistCounter(c *Counter, registry []OperationMeta) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	report := c.Report(registry, true)
	
	// 生成每日报告文件名
	filename := filepath.Join(p.dir, fmt.Sprintf("coverage_%s.json", time.Now().Format("2006-01-02")))
	
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	
	// 更新最后快照
	p.lastSnap = copyHits(report.Hits)
	
	return nil
}

// ListDailyReports lists all available daily reports
func (p *Persistence) ListDailyReports() ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	entries, err := os.ReadDir(p.dir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}
	
	var reports []string
	for _, e := range entries {
		if !e.IsDir() && len(e.Name()) >= 20 && e.Name()[:9] == "coverage_" {
			reports = append(reports, e.Name())
		}
	}
	return reports, nil
}

// LoadDailyReport loads a specific day's report
func (p *Persistence) LoadDailyReport(date string) (*DailyReport, error) {
	filename := filepath.Join(p.dir, fmt.Sprintf("coverage_%s.json", date))
	
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	
	var report DailyReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	
	return &report, nil
}

// GetLatestReport returns the most recent report
func (p *Persistence) GetLatestReport() (*DailyReport, error) {
	reports, err := p.ListDailyReports()
	if err != nil {
		return nil, err
	}
	if len(reports) == 0 {
		return nil, fmt.Errorf("no reports found")
	}
	
	// 返回最新的（按文件名排序最后一个）
	latest := reports[len(reports)-1]
	date := latest[9 : len(latest)-5]
	return p.LoadDailyReport(date)
}

// GetTrend calculates coverage trend over multiple days
func (p *Persistence) GetTrend(days int) ([]*DailyReport, error) {
	reports, err := p.ListDailyReports()
	if err != nil {
		return nil, err
	}
	
	if len(reports) == 0 {
		return nil, fmt.Errorf("no reports found")
	}
	
	// 按日期排序
	// 取最近的 days 天
	start := 0
	if len(reports) > days {
		start = len(reports) - days
	}
	
	var result []*DailyReport
	for i := start; i < len(reports); i++ {
		date := reports[i][9 : len(reports[i])-5]
		report, err := p.LoadDailyReport(date)
		if err != nil {
			continue
		}
		result = append(result, report)
	}
	
	return result, nil
}

// GenerateDailyReport generates and persists a daily report
func (p *Persistence) GenerateDailyReport(c *Counter, registry []OperationMeta) (*DailyReport, error) {
	report := c.Report(registry, true)
	
	// 填充日期
	dailyReport := &DailyReport{
		Date:            time.Now().Format("2006-01-02"),
		GeneratedAt:     time.Now(),
		OperationsTotal: report.OperationsTotal,
		OperationsHit:   report.OperationsHit,
		OperationsZero: len(report.OperationsZeroHit),
		CoverageRatio:  report.CoverageRatio,
		ZeroHitOps:     report.OperationsZeroHit,
		UnknownHits:    report.UnknownHits,
		Hits:           report.Hits,
	}
	
	// 持久化
	filename := filepath.Join(p.dir, fmt.Sprintf("coverage_%s.json", dailyReport.Date))
	data, err := json.MarshalIndent(dailyReport, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}
	
	return dailyReport, nil
}

func copyHits(hits map[string]uint64) map[string]uint64 {
	if hits == nil {
		return nil
	}
	result := make(map[string]uint64, len(hits))
	for k, v := range hits {
		result[k] = v
	}
	return result
}
