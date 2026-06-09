package exporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/devrix/devrix/internal/layers/observability/tracer"
)

// OTLPExporter exports spans via OTLP HTTP protocol (JSON format)
type OTLPExporter struct {
	endpoint    string
	serviceName string
	client      *http.Client
	timeout     time.Duration
}

// NewOTLPExporter creates a new OTLP exporter
func NewOTLPExporter(endpoint, serviceName string, timeout time.Duration) tracer.SpanExporter {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	if serviceName == "" {
		serviceName = "devrix"
	}
	return &OTLPExporter{
		endpoint:    endpoint,
		serviceName: serviceName,
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// OTLPResourceSpans is the OTLP JSON payload structure
type OTLPResourceSpans struct {
	ResourceSpans []OTLPResourceSpansV1 `json:"resourceSpans"`
}

// OTLPResourceSpansV1 is a single resource spans
type OTLPResourceSpansV1 struct {
	Resource   OTLPResource   `json:"resource"`
	ScopeSpans []OTLPScopeSpans `json:"scopeSpans"`
}

// OTLPResource is a resource
type OTLPResource struct {
	Attributes []OTLPKeyValue `json:"attributes"`
}

// OTLPKeyValue is a key-value pair
type OTLPKeyValue struct {
	Key   string      `json:"key"`
	Value OTLPAnyValue `json:"value"`
}

// OTLPAnyValue is a JSON value
type OTLPAnyValue struct {
	StringValue string `json:"stringValue,omitempty"`
}

// OTLPScopeSpans is a scope spans
type OTLPScopeSpans struct {
	Spans []OTLPSpan `json:"spans"`
}

// OTLPSpan is a span
type OTLPSpan struct {
	TraceID       string         `json:"traceId"`
	SpanID        string         `json:"spanId"`
	ParentSpanID  string         `json:"parentSpanId,omitempty"`
	Name          string         `json:"name"`
	Kind          int            `json:"kind"`
	StartTimeNano string         `json:"startTimeUnixNano"`
	EndTimeNano   string         `json:"endTimeUnixNano"`
	Attributes    []OTLPKeyValue `json:"attributes,omitempty"`
	Events        []OTLPEvent    `json:"events,omitempty"`
	Status        *OTLPStatus    `json:"status,omitempty"`
}

// OTLPEvent is a span event
type OTLPEvent struct {
	Name         string         `json:"name"`
	TimeUnixNano string         `json:"timeUnixNano"`
	Attributes   []OTLPKeyValue `json:"attributes,omitempty"`
}

// OTLPStatus is a span status
type OTLPStatus struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// Export exports a single span via OTLP HTTP
func (e *OTLPExporter) Export(ctx context.Context, s tracer.ReadableSpan) error {
	if s == nil {
		return nil
	}
	return e.ExportBatch(ctx, []tracer.ReadableSpan{s})
}

// ExportBatch exports multiple spans via OTLP HTTP
func (e *OTLPExporter) ExportBatch(ctx context.Context, spans []tracer.ReadableSpan) error {
	if len(spans) == 0 {
		return nil
	}

	// Build OTLP payload
	otlpSpans := make([]OTLPSpan, 0, len(spans))
	for _, s := range spans {
		if s != nil {
			otlpSpans = append(otlpSpans, e.spanToOTLP(s))
		}
	}

	payload := OTLPResourceSpans{
		ResourceSpans: []OTLPResourceSpansV1{
			{
				Resource: OTLPResource{
					Attributes: []OTLPKeyValue{
						{Key: "service.name", Value: OTLPAnyValue{StringValue: e.serviceName}},
					},
				},
				ScopeSpans: []OTLPScopeSpans{
					{Spans: otlpSpans},
				},
			},
		},
	}

	// Marshal to JSON
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal OTLP payload: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to export spans: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("OTLP exporter returned status %d: %s", resp.StatusCode, respBody)
	}

	return nil
}

// spanToOTLP converts a ReadableSpan to OTLP format
func (e *OTLPExporter) spanToOTLP(s tracer.ReadableSpan) OTLPSpan {
	sc := s.SpanContext()

	otlpSpan := OTLPSpan{
		TraceID:       sc.TraceID.String(),
		SpanID:        sc.SpanID.String(),
		Name:          s.Name(),
		Kind:          otlpSpanKind(s.Kind()),
		StartTimeNano: strconv.FormatInt(s.StartTime().UnixNano(), 10),
		EndTimeNano:   strconv.FormatInt(s.EndTime().UnixNano(), 10),
	}

	if parent := s.Parent(); parent != nil {
		otlpSpan.ParentSpanID = parent.SpanID.String()
	}

	// Convert attributes
	attrs := s.Attributes()
	otlpSpan.Attributes = make([]OTLPKeyValue, 0, len(attrs))
	for k, v := range attrs {
		otlpSpan.Attributes = append(otlpSpan.Attributes, OTLPKeyValue{
			Key:   k,
			Value: OTLPAnyValue{StringValue: fmt.Sprintf("%v", v)},
		})
	}

	// Convert events
	events := s.Events()
	otlpSpan.Events = make([]OTLPEvent, 0, len(events))
	for _, evt := range events {
		otlpSpan.Events = append(otlpSpan.Events, OTLPEvent{
			Name:         evt.Name,
			TimeUnixNano: strconv.FormatInt(evt.Timestamp.UnixNano(), 10),
			Attributes:   otlpEventAttributes(evt.Attributes),
		})
	}

	// Convert status
	status := s.Status()
	statusCode := 0 // Unset
	if status.Code == tracer.StatusCodeOk {
		statusCode = 1
	} else if status.Code == tracer.StatusCodeError {
		statusCode = 2
	}
	otlpSpan.Status = &OTLPStatus{
		Code:    statusCode,
		Message: status.Description,
	}

	return otlpSpan
}

func otlpSpanKind(kind tracer.SpanKind) int {
	// OpenTelemetry SpanKind enum: 1=INTERNAL, 2=SERVER, 3=CLIENT, 4=PRODUCER, 5=CONSUMER
	return int(kind) + 1
}

func otlpEventAttributes(attrs []tracer.Attribute) []OTLPKeyValue {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]OTLPKeyValue, 0, len(attrs))
	for _, attr := range attrs {
		out = append(out, OTLPKeyValue{
			Key:   attr.Key,
			Value: OTLPAnyValue{StringValue: fmt.Sprintf("%v", attr.Value)},
		})
	}
	return out
}

// Shutdown shuts down the exporter
func (e *OTLPExporter) Shutdown(_ context.Context) error {
	e.client.CloseIdleConnections()
	return nil
}
