package metrics

import (
	"context"
	"sync"
	"time"
)

// LSPMetrics DM-20260618-007 W3 (D2-S4-A01 / SUG-2) — LSP method 延迟
// 直方图 + 调用计数器 + 超时计数器, 由 lsptool_surface.Execute 触发。
//
// 注册到 D5 observability registry, 用于 p99 延迟告警 (默认 1500ms,
// devrix.yaml `lsp_latency_alert_ms` 可配置)。
type LSPMetrics struct {
	// Latency 按 method name 索引, 单方法 buckets: 5/10/25/50/100/250/500/1000/2500/5000 ms。
	Latency map[string]Histogram
	// Calls 按 method name 索引的总调用次数。
	Calls map[string]Counter
	// Timeouts 按 method name 索引的超时次数 (RequestTimeoutMs 触发)。
	Timeouts map[string]Counter
	mu       sync.RWMutex
}

// LSPMethodNames DM-20260618-007 W3 — 与 surface.LSPToolSurface 5 个 method 对齐。
var LSPMethodNames = []string{
	"lsp_go_to_definition",
	"lsp_find_references",
	"lsp_incoming_calls",
	"lsp_hover",
	"lsp_workspace_symbol",
}

// lspLatencyBuckets 延迟桶 (毫秒), 覆盖 ~1ms — 5s 范围, 适合 p99 告警 (1500ms 阈值)。
var lspLatencyBuckets = []float64{5, 10, 25, 50, 100, 250, 500, 1000, 1500, 2500, 5000}

// RegisterLSPMetrics creates and registers the LSP observability (D5)
// instruments bound to the given registry. Idempotent at registry level
// (RegisterHistogram/Counter 会覆盖同名). Safe to call multiple times
// across init paths.
func RegisterLSPMetrics(registry *Registry) *LSPMetrics {
	m := &LSPMetrics{
		Latency:  make(map[string]Histogram, len(LSPMethodNames)),
		Calls:    make(map[string]Counter, len(LSPMethodNames)),
		Timeouts: make(map[string]Counter, len(LSPMethodNames)),
	}
	for _, method := range LSPMethodNames {
		latLabels := LabelMap{"method": method}
		callLabels := LabelMap{"method": method}
		toLabels := LabelMap{"method": method}
		lat := NewHistogram("d2.lsp.latency.ms", latLabels, lspLatencyBuckets)
		c := NewCounter("d2.lsp.call.count", callLabels)
		t := NewCounter("d2.lsp.timeout.count", toLabels)
		_ = registry.RegisterHistogram("d2.lsp.latency.ms", latLabels, lat)
		_ = registry.RegisterCounter("d2.lsp.call.count", callLabels, c)
		_ = registry.RegisterCounter("d2.lsp.timeout.count", toLabels, t)
		m.Latency[method] = lat
		m.Calls[method] = c
		m.Timeouts[method] = t
	}
	return m
}

// LSPMethodTimer DM-20260618-007 W3 — 单次 LSP method 调用的延迟 timer。
// 用法: defer timer.Done(); 调用方在 done 前可以 Fail() 标记为失败 / 超时。
type LSPMethodTimer struct {
	metrics *LSPMetrics
	method  string
	start   time.Time
}

// StartLSPMethodTimer DM-20260618-007 W3 — 启动一个 method 计时器 (使用
// 默认 globalLSPMetrics 注册的 metrics; 若未注册则 no-op)。
//
// 调用方负责 defer timer.Done() 或 timer.Fail()。
func StartLSPMethodTimer(_ context.Context, method string) *LSPMethodTimer {
	return &LSPMethodTimer{
		metrics: globalLSPMetrics,
		method:  method,
		start:   time.Now(),
	}
}

// globalLSPMetrics DM-20260618-007 W3 — 全局 LSP metrics 单例, 由
// bootstrap.init() 调用 RegisterLSPMetrics 后赋值。nil 时 StartLSPMethodTimer
// 返回 no-op timer (不 panic)。
var globalLSPMetrics *LSPMetrics

// SetGlobalLSPMetrics DM-20260618-007 W3 — bootstrap 初始化 LSP metrics 后调用。
// 测试也可以用 (重置 + 重新注册)。
func SetGlobalLSPMetrics(m *LSPMetrics) { globalLSPMetrics = m }

// Done 标记 method 调用成功结束, 记录 latency + Inc call counter。
func (t *LSPMethodTimer) Done() { t.record(false) }

// Fail 标记 method 调用失败, 记录 latency + Inc call counter。
func (t *LSPMethodTimer) Fail() { t.record(false) }

// Timeout 标记 method 调用超时, 记录 latency + Inc call counter + Inc timeout counter。
func (t *LSPMethodTimer) Timeout() { t.record(true) }

// record 是内部统一入口。
func (t *LSPMethodTimer) record(isTimeout bool) {
	if t == nil || t.metrics == nil {
		return
	}
	t.metrics.mu.RLock()
	lat, lok := t.metrics.Latency[t.method]
	c, cok := t.metrics.Calls[t.method]
	t.metrics.mu.RUnlock()
	if !lok || !cok {
		return
	}
	lat.Observe(float64(time.Since(t.start).Milliseconds()))
	c.Inc()
	if isTimeout {
		t.metrics.mu.RLock()
		to, tok := t.metrics.Timeouts[t.method]
		t.metrics.mu.RUnlock()
		if tok {
			to.Inc()
		}
	}
}
