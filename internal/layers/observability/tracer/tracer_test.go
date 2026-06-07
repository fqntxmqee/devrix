package tracer

import (
	"testing"
)

func TestTraceIDGeneration(t *testing.T) {
	tid1 := GenerateTraceID()
	tid2 := GenerateTraceID()
	if tid1 == tid2 {
		t.Error("trace IDs should be unique")
	}
}

func TestSpanIDGeneration(t *testing.T) {
	sid1 := GenerateSpanID()
	sid2 := GenerateSpanID()
	if sid1 == sid2 {
		t.Error("span IDs should be unique")
	}
}
