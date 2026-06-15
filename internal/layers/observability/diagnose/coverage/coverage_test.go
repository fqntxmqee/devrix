package coverage_test

import (
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/diagnose/coverage"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
)

func TestCounter_should_record_hits_independently_of_sampling(t *testing.T) {
	t.Helper()
	ops := coverage.AllOperations()
	counter := coverage.NewCounter(ops)

	counter.RecordHit(telemetry.OpD1_S13_Capture_Message_Receive)
	counter.RecordHit(telemetry.OpD1_S13_Capture_Message_Receive)

	report := counter.Report(ops, true)
	if report.OperationsHit < 1 {
		t.Fatalf("expected gateway hit, report: %+v", report)
	}
	if report.Hits[telemetry.OpD1_S13_Capture_Message_Receive] != 2 {
		t.Fatalf("hits: %+v", report.Hits)
	}
}

func TestCounter_should_list_zero_hit_operations(t *testing.T) {
	t.Helper()
	ops := coverage.AllOperations()
	counter := coverage.NewCounter(ops)
	counter.RecordHit(telemetry.OpD2_S2_Context_Process)

	report := counter.Report(ops, false)
	if report.OperationsTotal != len(ops) {
		t.Fatalf("total %d, want %d", report.OperationsTotal, len(ops))
	}
	if report.OperationsHit != 1 {
		t.Fatalf("hit %d, want 1", report.OperationsHit)
	}
	if len(report.OperationsZeroHit) != len(ops)-1 {
		t.Fatalf("zero_hit len %d, want %d", len(report.OperationsZeroHit), len(ops)-1)
	}
}

func TestCounter_should_be_concurrent_safe(t *testing.T) {
	t.Helper()
	ops := coverage.AllOperations()
	counter := coverage.NewCounter(ops)

	const workers = 100
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			counter.RecordHit(telemetry.OpD3_S3_LLM_Stream)
		}()
	}
	wg.Wait()

	report := counter.Report(ops, true)
	if report.Hits[telemetry.OpD3_S3_LLM_Stream] != workers {
		t.Fatalf("hits %d, want %d", report.Hits[telemetry.OpD3_S3_LLM_Stream], workers)
	}
}
