package tracing

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TraceID represents a unique trace identifier
type TraceID string

// SpanID represents a unique span identifier
type SpanID string

// SpanKind represents the kind of span
type SpanKind string

const (
	// SpanKindClient indicates a client span
	SpanKindClient SpanKind = "client"

	// SpanKindServer indicates a server span
	SpanKindServer SpanKind = "server"

	// SpanKindInternal indicates an internal span
	SpanKindInternal SpanKind = "internal"

	// SpanKindProducer indicates a producer span
	SpanKindProducer SpanKind = "producer"

	// SpanKindConsumer indicates a consumer span
	SpanKindConsumer SpanKind = "consumer"
)

// Span represents a trace span
type Span struct {
	TraceID    TraceID           `json:"trace_id"`
	SpanID     SpanID            `json:"span_id"`
	ParentID   SpanID            `json:"parent_id,omitempty"`
	Name       string            `json:"name"`
	Kind       SpanKind          `json:"kind"`
	StartTime  time.Time         `json:"start_time"`
	EndTime    time.Time         `json:"end_time"`
	Duration   time.Duration     `json:"duration"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Events     []SpanEvent       `json:"events,omitempty"`
	Status     SpanStatus        `json:"status"`
	Error      string            `json:"error,omitempty"`
}

// SpanEvent represents an event within a span
type SpanEvent struct {
	Timestamp  time.Time         `json:"timestamp"`
	Name       string            `json:"name"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// SpanStatus represents span status
type SpanStatus string

const (
	// SpanStatusUnset indicates status is unset
	SpanStatusUnset SpanStatus = "unset"

	// SpanStatusOK indicates success
	SpanStatusOK SpanStatus = "ok"

	// SpanStatusError indicates an error
	SpanStatusError SpanStatus = "error"
)

// Tracer creates and manages spans
type Tracer struct {
	serviceName string
	spans       map[TraceID][]*Span
	mu          sync.RWMutex
	exporter    Exporter
}

// NewTracer creates a new tracer
func NewTracer(serviceName string) *Tracer {
	return &Tracer{
		serviceName: serviceName,
		spans:       make(map[TraceID][]*Span),
		exporter:    &NoOpExporter{},
	}
}

// SetExporter sets the span exporter
func (t *Tracer) SetExporter(exporter Exporter) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.exporter = exporter
}

// StartSpan starts a new span
func (t *Tracer) StartSpan(ctx context.Context, name string, kind SpanKind) (*Span, context.Context) {
	traceID := t.getOrCreateTraceID(ctx)
	parentID := t.getParentSpanID(ctx)

	span := &Span{
		TraceID:    traceID,
		SpanID:     t.generateSpanID(),
		ParentID:   parentID,
		Name:       name,
		Kind:       kind,
		StartTime:  time.Now(),
		Attributes: make(map[string]string),
		Status:     SpanStatusUnset,
	}

	// Add service name attribute
	span.Attributes["service.name"] = t.serviceName

	// Store span
	t.mu.Lock()
	t.spans[traceID] = append(t.spans[traceID], span)
	t.mu.Unlock()

	// Add span to context
	ctx = context.WithValue(ctx, traceIDKey{}, traceID)
	ctx = context.WithValue(ctx, spanIDKey{}, span.SpanID)

	return span, ctx
}

// EndSpan ends a span
func (t *Tracer) EndSpan(span *Span) {
	if span == nil {
		return
	}

	span.EndTime = time.Now()
	span.Duration = span.EndTime.Sub(span.StartTime)

	// Set status to OK if not already set
	if span.Status == SpanStatusUnset {
		span.Status = SpanStatusOK
	}

	// Export span
	if t.exporter != nil {
		t.exporter.ExportSpan(span)
	}
}

// SetSpanError sets span error status
func (t *Tracer) SetSpanError(span *Span, err error) {
	if span == nil || err == nil {
		return
	}

	span.Status = SpanStatusError
	span.Error = err.Error()
	span.Attributes["error"] = "true"
}

// AddSpanAttribute adds an attribute to span
func (t *Tracer) AddSpanAttribute(span *Span, key, value string) {
	if span == nil {
		return
	}

	span.Attributes[key] = value
}

// AddSpanEvent adds an event to span
func (t *Tracer) AddSpanEvent(span *Span, name string, attributes map[string]string) {
	if span == nil {
		return
	}

	event := SpanEvent{
		Timestamp:  time.Now(),
		Name:       name,
		Attributes: attributes,
	}

	span.Events = append(span.Events, event)
}

// GetTrace retrieves all spans for a trace
func (t *Tracer) GetTrace(traceID TraceID) []*Span {
	t.mu.RLock()
	defer t.mu.RUnlock()

	spans := t.spans[traceID]
	result := make([]*Span, len(spans))
	copy(result, spans)

	return result
}

// getOrCreateTraceID gets trace ID from context or creates new one
func (t *Tracer) getOrCreateTraceID(ctx context.Context) TraceID {
	if traceID, ok := ctx.Value(traceIDKey{}).(TraceID); ok {
		return traceID
	}

	return t.generateTraceID()
}

// getParentSpanID gets parent span ID from context
func (t *Tracer) getParentSpanID(ctx context.Context) SpanID {
	if spanID, ok := ctx.Value(spanIDKey{}).(SpanID); ok {
		return spanID
	}

	return ""
}

// generateTraceID generates a new trace ID
func (t *Tracer) generateTraceID() TraceID {
	return TraceID(fmt.Sprintf("trace-%d", time.Now().UnixNano()))
}

// generateSpanID generates a new span ID
func (t *Tracer) generateSpanID() SpanID {
	return SpanID(fmt.Sprintf("span-%d", time.Now().UnixNano()))
}

// Context keys
type traceIDKey struct{}
type spanIDKey struct{}

// Exporter exports spans to external system
type Exporter interface {
	ExportSpan(span *Span) error
}

// NoOpExporter does nothing with spans
type NoOpExporter struct{}

// ExportSpan implements Exporter interface
func (e *NoOpExporter) ExportSpan(span *Span) error {
	return nil
}

// InMemoryExporter stores spans in memory for testing
type InMemoryExporter struct {
	spans []*Span
	mu    sync.RWMutex
}

// NewInMemoryExporter creates a new in-memory exporter
func NewInMemoryExporter() *InMemoryExporter {
	return &InMemoryExporter{
		spans: make([]*Span, 0),
	}
}

// ExportSpan stores span in memory
func (e *InMemoryExporter) ExportSpan(span *Span) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.spans = append(e.spans, span)
	return nil
}

// GetSpans returns all stored spans
func (e *InMemoryExporter) GetSpans() []*Span {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Span, len(e.spans))
	copy(result, e.spans)

	return result
}

// Clear clears all stored spans
func (e *InMemoryExporter) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.spans = make([]*Span, 0)
}

// Global tracer
var globalTracer *Tracer
var globalTracerMu sync.RWMutex

// InitTracer initializes global tracer
func InitTracer(serviceName string) {
	globalTracerMu.Lock()
	defer globalTracerMu.Unlock()

	globalTracer = NewTracer(serviceName)
}

// GetTracer returns global tracer
func GetTracer() *Tracer {
	globalTracerMu.RLock()
	defer globalTracerMu.RUnlock()

	if globalTracer == nil {
		globalTracer = NewTracer("unknown")
	}

	return globalTracer
}

// StartSpan starts a span using global tracer
func StartSpan(ctx context.Context, name string, kind SpanKind) (*Span, context.Context) {
	return GetTracer().StartSpan(ctx, name, kind)
}

// EndSpan ends a span using global tracer
func EndSpan(span *Span) {
	GetTracer().EndSpan(span)
}

// SetSpanError sets span error using global tracer
func SetSpanError(span *Span, err error) {
	GetTracer().SetSpanError(span, err)
}

// AddSpanAttribute adds attribute using global tracer
func AddSpanAttribute(span *Span, key, value string) {
	GetTracer().AddSpanAttribute(span, key, value)
}

// AddSpanEvent adds event using global tracer
func AddSpanEvent(span *Span, name string, attributes map[string]string) {
	GetTracer().AddSpanEvent(span, name, attributes)
}
