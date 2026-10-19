package telemetry

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"sync"
	"time"
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
	span := &Span{
		TraceID:    traceID,
		SpanID:     randomHex(8),
		Name:       name,
		StartTime:  time.Now(),
		Status:     "ok",
		Attributes: make(map[string]any),
	}
	return context.WithValue(ctx, contextKey{}, span), span
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
	s.Attributes[key] = val
}

// RecordError marks the span as errored.
func (s *Span) RecordError(err error) {
	if s == nil || err == nil {
		return
	}
	s.Status = "error"
	s.Error = err.Error()
}

// End completes the span and records it into the collector.
func (s *Span) End() {
	if s == nil {
		return
	}
	s.EndTime = time.Now()
	s.DurationMs = s.EndTime.Sub(s.StartTime).Milliseconds()

	mu.Lock()
	defer mu.Unlock()
	spans = append(spans, *s)
}
