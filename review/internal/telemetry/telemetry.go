package telemetry

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type Span struct {
	TraceID    string         `json:"traceId"`
	SpanID     string         `json:"spanId"`
	Name       string         `json:"name"`
	StartTime  time.Time      `json:"startTime"`
	EndTime    time.Time      `json:"endTime"`
	DurationMs int64          `json:"durationMs"`
	Status     string         `json:"status"` // "ok" or "error"
	Error      string         `json:"error,omitempty"`
	Attributes map[string]any `json:"attributes"`
	otelSpan   trace.Span
	ctx        context.Context
}

type contextKey struct{}

var (
	mu          sync.Mutex
	forcedState *bool
	spans       []Span
)

// IsEnabled reports whether telemetry is active.
func IsEnabled() bool {
	mu.Lock()
	defer mu.Unlock()
	if forcedState != nil {
		return *forcedState
	}
	v := os.Getenv("OPEN_REVIEW_TELEMETRY")
	if v == "1" || strings.EqualFold(v, "true") {
		return true
	}
	o := os.Getenv("OPEN_REVIEW_OTEL")
	return o == "1" || strings.EqualFold(o, "true")
}

// SetEnabled explicitly enables or disables telemetry in process.
func SetEnabled(enabled bool) {
	mu.Lock()
	defer mu.Unlock()
	forcedState = &enabled
}

// Reset clears recorded spans and resets override state.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	forcedState = nil
	spans = nil
}

// Spans returns a copy of all recorded spans.
func Spans() []Span {
	mu.Lock()
	defer mu.Unlock()
	out := make([]Span, len(spans))
	copy(out, spans)
	return out
}

// Configure enables OTLP/HTTP export when telemetry is opted in and an OTLP
// endpoint is configured. Export failures never change review results.
func Configure(ctx context.Context) func(context.Context) {
	if !IsEnabled() || (os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT") == "") {
		return func(context.Context) {}
	}
	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return func(context.Context) {}
	}
	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		_ = traceExporter.Shutdown(ctx)
		return func(context.Context) {}
	}
	traces := sdktrace.NewTracerProvider(sdktrace.WithBatcher(traceExporter))
	metrics := sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)))
	otel.SetTracerProvider(traces)
	otel.SetMeterProvider(metrics)
	return func(shutdownCtx context.Context) {
		_ = errors.Join(metrics.Shutdown(shutdownCtx), traces.Shutdown(shutdownCtx))
	}
}

func randomHex(bytesLen int) string {
	b := make([]byte, bytesLen)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Start initiates a span if telemetry is enabled.
func Start(ctx context.Context, name string) (context.Context, *Span) {
	if !IsEnabled() {
		return ctx, nil
	}
	parent, _ := ctx.Value(contextKey{}).(*Span)
	traceID := ""
	if parent != nil {
		traceID = parent.TraceID
	} else {
		traceID = randomHex(16)
	}
	otelCtx, otelSpan := otel.Tracer("open-review").Start(ctx, name)
	span := &Span{
		TraceID:    traceID,
		SpanID:     randomHex(8),
		Name:       name,
		StartTime:  time.Now(),
		Status:     "ok",
		Attributes: make(map[string]any),
		otelSpan:   otelSpan,
		ctx:        otelCtx,
	}
	return context.WithValue(otelCtx, contextKey{}, span), span
}

var blockedSubstrings = []string{
	"diff",
	"patch",
	"source",
	"content",
	"secret",
	"password",
	"token_secret",
	"api_key",
	"apikey",
}

func isBlockedKey(key string) bool {
	lower := strings.ToLower(key)
	if lower == "prompt" || strings.HasSuffix(lower, ".prompt_text") || strings.HasSuffix(lower, "_prompt") {
		return true
	}
	for _, b := range blockedSubstrings {
		if strings.Contains(lower, b) {
			return true
		}
	}
	return false
}

// SetAttribute safely attaches operational attributes while filtering content.
func (s *Span) SetAttribute(key string, val any) {
	if s == nil {
		return
	}
	if isBlockedKey(key) {
		return // Policy: Omit source, diff, prompt, and secret content
	}
	switch key {
	case "model.provider", "model.name", "tool.name":
		value, ok := val.(string)
		if !ok || len(value) > 120 || strings.ContainsAny(value, " \r\n\t@") {
			return
		}
	case "tokens.prompt", "tokens.output":
		if _, ok := val.(int); !ok {
			return
		}
	default:
		return
	}
	s.Attributes[key] = val
}

// RecordError marks the span as errored.
func (s *Span) RecordError(err error) {
	if s == nil || err == nil {
		return
	}
	s.Status = "error"
	s.Error = "operation_failed"
}

// End completes the span and records it into the collector.
func (s *Span) End() {
	if s == nil {
		return
	}
	s.EndTime = time.Now()
	s.DurationMs = s.EndTime.Sub(s.StartTime).Milliseconds()
	if s.otelSpan != nil {
		for key, val := range s.Attributes {
			switch typed := val.(type) {
			case string:
				s.otelSpan.SetAttributes(attribute.String(key, typed))
			case int:
				s.otelSpan.SetAttributes(attribute.Int(key, typed))
			}
		}
		if s.Status == "error" {
			s.otelSpan.SetStatus(codes.Error, "operation_failed")
		}
		s.otelSpan.End()
	}
	meter := otel.Meter("open-review")
	if duration, err := meter.Int64Histogram("open_review.operation.duration", otelmetric.WithUnit("ms")); err == nil {
		duration.Record(s.ctx, s.DurationMs, otelmetric.WithAttributes(attribute.String("operation", s.Name)))
	}
	if tokens, err := meter.Int64Counter("open_review.model.tokens"); err == nil {
		for _, part := range []string{"prompt", "output"} {
			if count, ok := s.Attributes["tokens."+part].(int); ok {
				tokens.Add(s.ctx, int64(count), otelmetric.WithAttributes(attribute.String("kind", part)))
			}
		}
	}

	mu.Lock()
	defer mu.Unlock()
	spans = append(spans, *s)
}
