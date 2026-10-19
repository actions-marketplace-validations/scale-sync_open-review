package telemetry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTelemetryDisabledByDefault(t *testing.T) {
	Reset()
	t.Setenv("OPEN_REVIEW_TELEMETRY", "")
	t.Setenv("OPEN_REVIEW_OTEL", "")
	if IsEnabled() {
		t.Fatal("telemetry should be disabled by default")
	}
	ctx, span := Start(context.Background(), "test")
	if span != nil {
		t.Fatal("span should be nil when disabled")
	}
	_ = ctx
}

func TestTelemetryEnabledAndContentRedaction(t *testing.T) {
	Reset()
	defer Reset()
	SetEnabled(true)
	if !IsEnabled() {
		t.Fatal("telemetry should be enabled")
	}

	ctx, span := Start(context.Background(), "model_call")
	span.SetAttribute("model.provider", "azure")
	span.SetAttribute("model.name", "gpt-4o")
	span.SetAttribute("tokens.prompt", 150)
	span.SetAttribute("tokens.output", 45)

	// Blocked content keys must not be stored
	span.SetAttribute("prompt", "secret prompt text")
	span.SetAttribute("diff", "diff --git code")
	span.SetAttribute("user_api_key", "sk-12345678901234567890")

	time.Sleep(2 * time.Millisecond)
	span.End()

	records := Spans()
	if len(records) != 1 {
		t.Fatalf("recorded spans count: %d", len(records))
	}
	rec := records[0]
	if rec.Name != "model_call" || rec.Status != "ok" || rec.DurationMs < 0 {
		t.Fatalf("unexpected span: %+v", rec)
	}
	if rec.Attributes["model.provider"] != "azure" || rec.Attributes["tokens.prompt"] != 150 {
		t.Fatalf("attributes missing: %+v", rec.Attributes)
	}
	if _, ok := rec.Attributes["prompt"]; ok {
		t.Fatal("prompt content was not redacted/blocked")
	}
	if _, ok := rec.Attributes["diff"]; ok {
		t.Fatal("diff content was not redacted/blocked")
	}
	if _, ok := rec.Attributes["user_api_key"]; ok {
		t.Fatal("api key was not redacted/blocked")
	}

	// Test Child Span with parent TraceID
	_, childSpan := Start(ctx, "tool_call")
	childSpan.RecordError(errors.New("sandbox timeout"))
	childSpan.End()

	all := Spans()
	if len(all) != 2 {
		t.Fatalf("total spans: %d", len(all))
	}
	childRec := all[1]
	if childRec.TraceID != rec.TraceID {
		t.Fatalf("child TraceID %s != parent TraceID %s", childRec.TraceID, rec.TraceID)
	}
	if childRec.Status != "error" || childRec.Error != "sandbox timeout" {
		t.Fatalf("child error not captured: %+v", childRec)
	}
}
