package observability

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/coverage"
)

func TestCoverageIntegration_染色是否工作(t *testing.T) {
	ctx := context.Background()
	
	// 初始化
	obs, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer obs.Shutdown(ctx)

	// 检查 coverage counter
	counter := coverage.Global()
	if counter == nil {
		t.Fatal("Coverage counter is nil")
	}

	// 获取 tracer
	tracer := obs.Tracer()
	if tracer == nil {
		t.Fatal("Tracer is nil")
	}

	t.Log("创建 spans 来测试染色...")

	// 创建一些 span，触发染色
	testOps := []string{
		"context.process",
		"context.pev.run",
		"context.pev.llm_call",
		"context.pev.verify",
		"unknown.operation", // 测试 unknown
	}

	for _, op := range testOps {
		_, span := tracer.Start(ctx, op)
		if span != nil {
			span.End()
			t.Logf("  Created span: %s", op)
		}
	}

	// 生成报告
	report := counter.Report(coverage.AllOperations(), true)

	t.Logf("=== Coverage Report ===")
	t.Logf("Total operations: %d", report.OperationsTotal)
	t.Logf("Hit operations: %d", report.OperationsHit)
	t.Logf("Coverage ratio: %.2f%%", report.CoverageRatio*100)
	t.Logf("Zero hit operations: %d", len(report.OperationsZeroHit))

	// 显示命中的操作
	if len(report.Hits) > 0 {
		t.Log("=== Hit Operations ===")
		for op, count := range report.Hits {
			t.Logf("  %s: %d hits", op, count)
		}
	}

	// 验证我们的测试操作被记录了
	for _, op := range testOps {
		if op == "unknown.operation" {
			continue // 这个不在 registry 中
		}
		if count, ok := report.Hits[op]; !ok || count == 0 {
			t.Errorf("Expected operation %s to be hit, but got count: %d", op, count)
		}
	}

	// 测试未知操作
	unknownCount := report.UnknownHits
	t.Logf("Unknown operations count: %d (未在 registry 中注册的操作)", unknownCount)
}

// TestCoverageReport_查看完整报告 runs a full coverage report
func TestCoverageReport_查看完整报告(t *testing.T) {
	ctx := context.Background()
	obs, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer obs.Shutdown(ctx)

	counter := coverage.Global()
	if counter == nil {
		t.Fatal("Coverage counter is nil")
	}

	// 模拟一些常见操作
	tracer := obs.Tracer()
	if tracer != nil {
		ops := []string{
			"context.process",
			"context.pev.run",
			"context.pev.llm_call",
			"context.pev.tool_execute",
			"context.pev.verify",
			"context.compression.run",
			"llm.stream",
		}
		for _, op := range ops {
			_, span := tracer.Start(ctx, op)
			if span != nil {
				span.End()
			}
		}
	}

	report := counter.Report(coverage.AllOperations(), true)

	t.Log("\n========== Coverage Report ==========")
	t.Logf("Total: %d operations", report.OperationsTotal)
	t.Logf("Hit: %d operations", report.OperationsHit)
	t.Logf("Coverage: %.1f%%", report.CoverageRatio*100)
	t.Logf("Zero-hit: %d operations", len(report.OperationsZeroHit))
	t.Log("========================================")
	
	if len(report.OperationsZeroHit) > 0 {
		t.Log("\nZero-hit operations (可能的无用代码):")
		for _, entry := range report.OperationsZeroHit {
			t.Logf("  [%s.%s] %s (since %s)", 
				entry.Layer, entry.Component, entry.Operation, entry.SinceVersion)
		}
	}
}
