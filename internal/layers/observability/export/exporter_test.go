package export

import (
	"testing"
)

func TestConsoleExporter_should_create_instance(t *testing.T) {
	exporter := NewConsoleExporter()
	if exporter == nil {
		t.Fatal("should create exporter")
	}
	if exporter.Shutdown(nil) != nil {
		t.Fatal("shutdown should succeed")
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
