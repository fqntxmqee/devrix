package exporter

import (
	"testing"
)

func TestConsoleExporter(t *testing.T) {
	exporter := NewConsoleExporter()
	if exporter == nil {
		t.Fatal("should create exporter")
	}
}

func TestOTLPExporter(t *testing.T) {
	exporter := NewOTLPExporter("http://localhost:4318", "devrix", 0)
	if exporter == nil {
		t.Fatal("should create OTLP exporter")
	}
}

func TestNullExporter(t *testing.T) {
	exporter := NewNullExporter()
	if exporter == nil {
		t.Fatal("should create null exporter")
	}
}
